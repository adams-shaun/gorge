package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExiledWithOwnerRefSeat pins RefExiledWithOwner's read: the first seat
// in seat order whose exile zone holds a card exiled with the reading source
// (gorge DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1, the
// shared player filter's HasCardsIn property). The read is exwHas -- the ONE
// reader exwCount and PredExiledWith share -- so an unlinked card never
// matches and 255 means nobody.
func TestExiledWithOwnerRefSeat(t *testing.T) {
	sc, _, c := exwStatScen(t)
	s := sc.s
	host := sc.perm(c["host"], 0)
	ctx := evalCtx{src: host, srcSl: sc.slot(host), ctrl: 0}
	if got := s.exiledWithOwnerSeat(ctx); got != 255 {
		t.Fatalf("nothing exiled: seat %d, want 255", got)
	}
	// A card in exile exiled by nothing is not a card exiled with the source.
	exwExile(sc, c["golem"], 0, NoObj)
	if got := s.exiledWithOwnerSeat(ctx); got != 255 {
		t.Fatalf("unlinked card in exile: seat %d, want 255", got)
	}
	// The other seat's exile zone is read too (gorge walks every seat).
	exwExile(sc, c["bear"], 1, host)
	if got := s.exiledWithOwnerSeat(ctx); got != 1 {
		t.Fatalf("bear exiled in seat 1's exile: seat %d, want 1", got)
	}
	// Several linked cards in ONE seat's exile: gorge asks that seat once.
	exwExile(sc, c["land"], 1, host)
	if got := s.exiledWithOwnerSeat(ctx); got != 1 {
		t.Fatalf("two linked cards in seat 1's exile: seat %d, want 1", got)
	}
	if r := s.StallReason(); r != StallNone {
		t.Fatalf("one seat holding every linked card stalled with %d, want none", r)
	}
}

// TestExiledWithOwnerMultiSeatStalls pins the guard: linked cards in two
// seats' exile zones are a shape RefExiledWithOwner (one seat) cannot
// represent while gorge acts on every matching seat, so the read latches
// StallLowering and still returns the first seat in seat order.
func TestExiledWithOwnerMultiSeatStalls(t *testing.T) {
	sc, _, c := exwStatScen(t)
	s := sc.s
	host := sc.perm(c["host"], 0)
	ctx := evalCtx{src: host, srcSl: sc.slot(host), ctrl: 0}
	exwExile(sc, c["bear"], 1, host)
	if got := s.exiledWithOwnerSeat(ctx); got != 1 || s.StallReason() != StallNone {
		t.Fatalf("one linked seat: seat %d stall %d, want seat 1 and no stall", got, s.StallReason())
	}
	exwExile(sc, c["land"], 0, host)
	if got := s.exiledWithOwnerSeat(ctx); got != 0 {
		t.Fatalf("two linked seats: seat %d, want 0 (first in seat order)", got)
	}
	if r := s.StallReason(); r != StallLowering {
		t.Fatalf("two linked seats: stall %d, want StallLowering %d", r, StallLowering)
	}
}

// dreadOwnerScen is the Hovership's ETB->leave shape: a card whose program
// first exiles its target and then manifests dread for the exiled card's
// owner. reader is the card's own ExiledWith / ExiledWithRev mark (kgen's
// lowering sets both); without them the kernel's exwRec gate records no
// link. A second reader keeps genPrep.anyExW on when the source's bits are
// cleared, so the negative case reaches exwExile's per-card exwRec gate
// rather than skipping the link code entirely.
func dreadOwnerScen(t *testing.T, reader bool) (sc *scen, src, bear Obj, prog []kop.Instr) {
	t.Helper()
	var srcCard, bearCard CardID
	sc, _ = newSetScenTweak(t, "fdn", []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		srcCard = CardID(len(tbl.Cards))
		bearCard = srcCard + 1
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "T Dread Source", ExiledWith: reader, ExiledWithRev: reader,
				Types: kop.TCreature, Power: 3, Toughness: 3,
				Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
					{Op: kop.OpExile, Obj: kop.RefTarget0},
					{Op: kop.OpManifestDread, Obj: kop.RefExiledWithOwner, A: kop.Const(1)},
				}}}},
			kop.CardDef{Name: "T Dread Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
			kop.CardDef{Name: "T Other Reader", ExiledWith: true, ExiledWithRev: true,
				Types: kop.TCreature, Power: 1, Toughness: 1})
	})
	src = sc.perm(srcCard, 0)
	bear = sc.perm(bearCard, 1)
	prog = sc.k.t.Cards[srcCard].Abilities[0].Prog
	return sc, src, bear, prog
}

// dreadResolve seats the scenario's program as the resolving entry (source
// src, controller 0, ability 0), with bear as its target.
func dreadResolve(sc *scen, src, bear Obj) {
	s := sc.s
	s.res.active, s.res.e.obj, s.res.e.ctrl, s.res.e.ab = true, src, 0, 0
	s.res.e.card = s.objs[src].card
	s.res.e.nt, s.res.e.tgt[0], s.res.e.tver[0] = 1, TCode(bear), s.objs[bear].ver
	s.res.pc, s.res.ask, s.res.askP = 0, raNone, 0
	s.sp = 1
}

// TestExiledWithOwnerManifestDread drives the whole ETB->leave sequence
// through the kernel, from a real OpExile: the exile records the link only
// because the card's reader bits (genPrep.exwRec) are set, and the leave
// op's RefExiledWithOwner then asks the exiled card's owner's seat. With no
// link it manifests dread for NOBODY -- never falling back to the resolving
// controller (gorge effManifestDread over searchPlayers's empty set).
func TestExiledWithOwnerManifestDread(t *testing.T) {
	sc, src, bear, prog := dreadOwnerScen(t, true)
	s := sc.s
	dreadResolve(sc, src, bear)
	// With no link yet: the leave op completes with no ask and no manifest.
	if !s.exec(sc.k, &prog[1]) {
		t.Fatalf("empty-exile op suspended (ask %d), want it to complete", s.res.ask)
	}
	if s.res.ask != raNone {
		t.Fatalf("empty-exile ask %d, want none", s.res.ask)
	}
	if s.res.askP != 0 {
		t.Fatalf("empty-exile ask seat %d, want none (0)", s.res.askP)
	}
	// The ETB exiles the bear through OpExile; the link is the kernel's own
	// record, not a seeded one.
	s.res.pc, s.res.ask, s.res.askP = 0, raNone, 0
	if !s.exec(sc.k, &prog[0]) {
		t.Fatalf("ETB exile suspended (ask %d), want it to complete", s.res.ask)
	}
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("ETB sent the bear to zone %d, want exile %d", z, kop.ZoneExile)
	}
	if got := s.exiledWithOwnerSeat(evalCtx{src: src, srcSl: sc.slot(src), ctrl: 0}); got != 1 {
		t.Fatalf("after the ETB the linked card reads seat %d, want 1", got)
	}
	// The linked card: the ask is seat 1's manifest dread.
	s.res.pc, s.res.ask, s.res.askP = 1, raNone, 0
	if s.exec(sc.k, &prog[1]) || s.res.ask != raGenManifestDread {
		t.Fatalf("ask %d, want seat 1's manifest dread", s.res.ask)
	}
	if s.res.askP != 1 {
		t.Fatalf("ask seat %d, want 1 (the linked card's owner)", s.res.askP)
	}
}

// TestExiledWithOwnerWithoutReaderBitsRecordsNothing is the other side of
// the same kernel gate: the identical ETB with a card that lacks the
// ExiledWith/ExiledWithRev marks (genPrep.exwRec stays zero) records no
// link, so the leave op reads 255 and asks nobody. That is the state the
// Hovership lowering produced before the reader bits were set.
func TestExiledWithOwnerWithoutReaderBitsRecordsNothing(t *testing.T) {
	sc, src, bear, prog := dreadOwnerScen(t, false)
	s := sc.s
	dreadResolve(sc, src, bear)
	if !s.exec(sc.k, &prog[0]) {
		t.Fatalf("ETB exile suspended (ask %d), want it to complete", s.res.ask)
	}
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("ETB sent the bear to zone %d, want exile %d", z, kop.ZoneExile)
	}
	if got := s.exiledWithOwnerSeat(evalCtx{src: src, srcSl: sc.slot(src), ctrl: 0}); got != 255 {
		t.Fatalf("without the reader bits the link reads seat %d, want 255 (none)", got)
	}
	s.res.pc, s.res.ask, s.res.askP = 1, raNone, 0
	if !s.exec(sc.k, &prog[1]) || s.res.ask != raNone {
		t.Fatalf("no link: op suspended with ask %d, want it to complete with no ask", s.res.ask)
	}
}
