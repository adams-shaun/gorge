package dirac

import (
	"testing"

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
	// A second linked card in seat 0's exile wins on seat order.
	exwExile(sc, c["land"], 0, host)
	if got := s.exiledWithOwnerSeat(ctx); got != 0 {
		t.Fatalf("land exiled in seat 0's exile: seat %d, want 0 (seat order)", got)
	}
}

// TestExiledWithOwnerManifestDread: OpManifestDread over RefExiledWithOwner
// manifests dread for the linked card's owner's seat, and when no seat's
// exile holds one it manifests dread for NOBODY -- never falling back to the
// resolving controller (gorge effManifestDread over searchPlayers's empty
// set).
func TestExiledWithOwnerManifestDread(t *testing.T) {
	sc, _, c := exwStatScen(t)
	s := sc.s
	src := sc.perm(c["host"], 0)
	prog := []kop.Instr{{Op: kop.OpManifestDread, Obj: kop.RefExiledWithOwner, A: kop.Const(1)}}
	sc.k.t.Cards = append(sc.k.t.Cards, kop.CardDef{Name: "Test Dread Owner", Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	s.res.active, s.res.e.obj, s.res.e.ctrl, s.res.e.card, s.res.e.ab, s.res.pc = true, src, 0, c["host"], 0, 0
	s.sp = 1
	// With no link: the op completes with no ask and no manifest.
	in := &prog[0]
	if !s.exec(sc.k, in) {
		t.Fatalf("empty-exile op suspended (ask %d), want it to complete", s.res.ask)
	}
	if s.res.ask != raNone {
		t.Fatalf("empty-exile ask %d, want none", s.res.ask)
	}
	if s.res.askP != 0 {
		t.Fatalf("empty-exile ask seat %d, want none (0)", s.res.askP)
	}
	// Seat 1's exile then holds a card linked to the source (the manual exile
	// runs with res.active down, as the other exw tests place their cards: an
	// active resolution would route the place through the entry's program).
	s.res.active = false
	exwExile(sc, c["bear"], 1, src)
	s.res.active = true
	// The linked card: the ask is seat 1's manifest dread.
	s.res.pc, s.res.ask, s.res.askP = 0, raNone, 0
	in = &prog[0]
	if s.exec(sc.k, in) || s.res.ask != raGenManifestDread {
		t.Fatalf("ask %d, want seat 1's manifest dread", s.res.ask)
	}
	if s.res.askP != 1 {
		t.Fatalf("ask seat %d, want 1 (the linked card's owner)", s.res.askP)
	}
}
