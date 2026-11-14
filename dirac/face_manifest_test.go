package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newManifestScen is newScenTable over the kmanifest pool: Ornithopter (a
// {0}-cost creature) and Dryad Arbor (a "no cost" creature) beside the
// carriers the zero-cost turn-up tests need.
func newManifestScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Ornithopter", Count: 1},
		{Name: "Dryad Arbor", Count: 1},
		{Name: "Soul Summons", Count: 1},
		{Name: "Cryptic Coat", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
	}
	set := cardsettest.Set(t, "manifest")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib := make([]CardID, 20)
	for i := range lib {
		lib[i] = id["Ornithopter"]
	}
	s := new(State)
	s.Init(tbl, lib, lib, 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// topCard swaps the card of the top library object for c (the object stays
// in the library, so no zone bookkeeping changes).
func topCard(sc *scen, c CardID) Obj {
	pl := &sc.s.pl[0]
	o := pl.lib[pl.libLo]
	sc.s.objs[o].card = c
	return o
}

// Manifest moves the top card of the named player's library onto the
// battlefield face down as itself, marked manifested (CR 701.40a).
func TestManifestMovesTopCardFaceDown(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	o := topCard(sc, id["Kolaghan Stormsinger"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	if s.objs[o].zone != kop.ZoneBattlefield || !s.faceDown(sc.k, s.objs[o].slot) || !s.manifested(s.objs[o].slot) {
		t.Fatalf("top card zone %d faceDown %v manifested %v", s.objs[o].zone,
			s.faceDown(sc.k, s.objs[o].slot), s.manifested(s.objs[o].slot))
	}
	if s.objs[o].acts>>7 != 0 {
		t.Fatal("manifested under the wrong controller")
	}
}

// A cloaked permanent is face down with ward {2} (CR 701.58a) and the
// cloaked flag makes the turn-face-up walk offer its mana cost and its
// keyword alternative.
func TestCloakWardAndTurnUpOffers(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	o := topCard(sc, id["Kolaghan Stormsinger"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpCloak, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(kop.CfTapped)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Cloak suspended")
	}
	sl := s.objs[o].slot
	if !s.cloaked(sl) || !s.faceDown(sc.k, sl) {
		t.Fatal("not a cloaked face-down permanent")
	}
	if s.perms[sl].flags&pfTapped == 0 {
		t.Fatal("CfTapped did not tap it")
	}
	if kw := s.kw(sc.k, sl); kw&kop.KwWard == 0 {
		t.Fatalf("cloaked keyword %#x lacks ward", kw)
	}
	s.pl[0].pool[kop.R] = 1
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	modes := map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		modes[d.Options[i].Mode()] = true
	}
	if !modes[2] || !modes[4] {
		t.Fatalf("turn-up modes %v, want cloak (2) and megamorph (4)", modes)
	}
	s.turnFaceUp(sc.k, o, 0, 2)
	if s.faceDown(sc.k, sl) || s.cloaked(sl) || s.perms[sl].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatal("cloak mana turn-up left face-down/cloaked or made a megamorph counter")
	}
}

// Manifest dread looks at the top two cards, manifests the chosen one and
// puts the other into its owner's graveyard (CR 701.62a).
func TestManifestDreadAskAndSplit(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	pl := &s.pl[0]
	keep := topCard(sc, id["Kolaghan Stormsinger"])
	pl.libLo++
	other := topCard(sc, id["Bolrac-Clan Basher"])
	pl.libLo--
	prog := []kop.Instr{{Op: kop.OpManifestDread, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(0)}}
	src := CardID(len(sc.k.t.Cards))
	sc.k.t.Cards = append(sc.k.t.Cards, kop.CardDef{Name: "Test Dread", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	s.res.e.ctrl, s.res.e.card, s.res.e.ab, s.res.pc = 0, src, 0, 0
	s.sp = 1
	in := &prog[0]
	if s.exec(sc.k, in) || s.res.ask != raGenManifestDread {
		t.Fatalf("ask %d, want manifest dread", s.res.ask)
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if d.Sub != ChGenManifestDread || d.N != 2 {
		t.Fatalf("manifest dread ask: sub %d with %d options", d.Sub, d.N)
	}
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(other)) {
			pick = d.Options[i]
		}
	}
	if pick.Kind() != AkChoose {
		t.Fatal("the other top card is not offered")
	}
	s.applyAskOblige(sc.k, pick)
	if s.objs[other].zone != kop.ZoneBattlefield || !s.manifested(s.objs[other].slot) {
		t.Fatalf("chosen card zone %d", s.objs[other].zone)
	}
	if s.objs[keep].zone != kop.ZoneGraveyard {
		t.Fatalf("unchosen card zone %d, want graveyard", s.objs[keep].zone)
	}
}

// A manifested noncreature card never gets the mana-cost procedure
// (CR 701.40b needs a creature card) and has no Morph-family alternative.
func TestManifestedNoncreatureNever(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	topCard(sc, id["Shock"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	s.pl[0].pool[kop.R] = 1
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	if d.N != 0 {
		t.Fatalf("manifested Shock offered %d turn-ups, want none", d.N)
	}
}

// A manifested card that prints Morph offers both the printed mana cost
// (mode 1) and the keyword alternative (fam+2), CR 701.40b-d.
func TestManifestedMorphBothCosts(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	topCard(sc, id["Kolaghan Stormsinger"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	s.pl[0].pool[kop.R] = 5
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	modes := map[uint8]bool{}
	for i := 0; i < int(d.N); i++ {
		modes[d.Options[i].Mode()] = true
	}
	if !modes[1] || !modes[4] {
		t.Fatalf("turn-up modes %v, want mana cost (1) and megamorph (4)", modes)
	}
}

// A generic face-down entry carries no manifested/cloaked provenance and a
// card with no Morph family has no turn-up procedure (CR 708.2a).
func TestGenericFaceDownNoTurnUp(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	o := topCard(sc, id["Zoetic Cavern"])
	s.detach(o)
	sl := s.enterFaceDown(sc.k, o, 0, 0, kop.ZoneLibrary)
	if s.manifested(sl) || s.cloaked(sl) {
		t.Fatal("generic face-down entry took provenance")
	}
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	if d.N != 0 {
		t.Fatalf("generic face-down land offered %d turn-ups, want none", d.N)
	}
}

// A manifested creature card turns face up for its printed mana cost
// (CR 701.40b) and the mana-cost procedure is not the Megamorph one, so it
// adds no +1/+1 counter.
func TestManifestedCreatureTurnsUp(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	o := topCard(sc, id["Kolaghan Stormsinger"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	sl := s.objs[o].slot
	s.pl[0].pool[kop.R] = 1
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == 1 {
			pick = d.Options[i]
		}
	}
	if pick.Kind() != AkTurnFaceUp {
		t.Fatalf("manifested creature offered no mana-cost turn-up (%d options)", d.N)
	}
	s.turnFaceUp(sc.k, o, 0, 1)
	if s.faceDown(sc.k, sl) || s.manifested(sl) {
		t.Fatal("manifested creature stayed face down")
	}
	if s.perms[sl].ctrLow(int(kop.CtrP1P1)) != 0 {
		t.Fatal("the mana-cost turn-up added a Megamorph counter")
	}
}

// A printed {0} mana cost IS a mana cost (CostEmpty is false), so a
// manifested Ornithopter offers the printed-cost turn-up (CR 701.40b).
func TestManifestedZeroCostTurnsUp(t *testing.T) {
	sc, id := newManifestScen(t)
	s := sc.s
	o := topCard(sc, id["Ornithopter"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	sl := s.objs[o].slot
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	found := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifested {0}-cost creature offered no mana-cost turn-up (%d options)", d.N)
	}
	s.turnFaceUp(sc.k, o, 0, 1)
	if s.faceDown(sc.k, sl) || s.manifested(sl) {
		t.Fatal("manifested {0}-cost creature stayed face down")
	}
}

// A printed "no cost" (Dryad Arbor) prices as a free mana cost in gorge
// (manifestFaceUpCost rejects only EMPTY cost text; ParseCost("no cost") is
// the free cost), so a manifested one offers the printed-cost procedure and
// turns up for nothing.
func TestManifestedNoCostTurnsUpFree(t *testing.T) {
	sc, id := newManifestScen(t)
	s := sc.s
	o := topCard(sc, id["Dryad Arbor"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	sl := s.objs[o].slot
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	found := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifested no-cost creature offered no mana-cost turn-up (%d options)", d.N)
	}
	s.turnFaceUp(sc.k, o, 0, 1)
	if s.faceDown(sc.k, sl) || s.manifested(sl) {
		t.Fatal("manifested no-cost creature stayed face down")
	}
}

// A cloaked creature card turns face up for its printed mana cost
// (CR 701.58b); turning up clears the cloaked provenance, so its ward {2}
// is gone.
func TestCloakedCreatureTurnsUp(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	o := topCard(sc, id["Kolaghan Stormsinger"])
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpCloak, Obj: kop.RefController, A: kop.Const(1)}
	if !s.exec(sc.k, &in) {
		t.Fatal("Cloak suspended")
	}
	sl := s.objs[o].slot
	if kw := s.kw(sc.k, sl); kw&kop.KwWard == 0 {
		t.Fatal("cloaked permanent lacks ward")
	}
	s.pl[0].pool[kop.R] = 1
	var d Decision
	s.offerTurnFaceUp(sc.k, &d, 0)
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == 2 {
			pick = d.Options[i]
		}
	}
	if pick.Kind() != AkTurnFaceUp {
		t.Fatalf("cloaked creature offered no mana-cost turn-up (%d options)", d.N)
	}
	s.turnFaceUp(sc.k, o, 0, 2)
	if s.faceDown(sc.k, sl) || s.cloaked(sl) {
		t.Fatal("cloaked creature stayed face down")
	}
	if kw := s.kw(sc.k, sl); kw&kop.KwWard != 0 {
		t.Fatal("turned-up cloak kept ward {2}")
	}
}

// A card manifested by ManifestDread is manifested like any other and turns
// face up for its mana cost (gorge's TestManifestDreadCreatureTurnsUp).
func TestManifestDreadCreatureTurnsUp(t *testing.T) {
	sc, id := newKfacedownScen(t)
	s := sc.s
	pl := &s.pl[0]
	chosen := topCard(sc, id["Kolaghan Stormsinger"])
	pl.libLo++
	topCard(sc, id["Bolrac-Clan Basher"])
	pl.libLo--
	prog := []kop.Instr{{Op: kop.OpManifestDread, Obj: kop.RefController, A: kop.Const(1), B: kop.Const(0)}}
	src := CardID(len(sc.k.t.Cards))
	sc.k.t.Cards = append(sc.k.t.Cards, kop.CardDef{Name: "Test Dread", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: prog}}})
	s.res.e.ctrl, s.res.e.card, s.res.e.ab, s.res.pc = 0, src, 0, 0
	s.sp = 1
	if s.exec(sc.k, &prog[0]) || s.res.ask != raGenManifestDread {
		t.Fatalf("ask %d, want manifest dread", s.res.ask)
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	var pick Action
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Source() == TCode(uint8(chosen)) {
			pick = d.Options[i]
		}
	}
	if pick.Kind() != AkChoose {
		t.Fatal("the chosen top card is not offered")
	}
	s.applyAskOblige(sc.k, pick)
	sl := s.objs[chosen].slot
	if s.objs[chosen].zone != kop.ZoneBattlefield || !s.manifested(sl) {
		t.Fatalf("dread-manifested card zone %d", s.objs[chosen].zone)
	}
	s.pl[0].pool[kop.R] = 1
	var up Decision
	s.offerTurnFaceUp(sc.k, &up, 0)
	found := false
	for i := 0; i < int(up.N); i++ {
		if up.Options[i].Mode() == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("dread-manifested creature offered no mana-cost turn-up (%d options)", up.N)
	}
	s.turnFaceUp(sc.k, chosen, 0, 1)
	if s.faceDown(sc.k, sl) || s.manifested(sl) {
		t.Fatal("dread-manifested creature stayed face down")
	}
}
