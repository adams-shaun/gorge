package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDynTriggerCostPayIdentityEmptyExtension is the commander-identity shape
// beside a dynamic-amount source, with no commander on the board: an
// Arcane Signet/Command Tower-style "one mana of any colour in your
// commander's colour identity" source reads no colour when the seat has no
// commander (CR 903.4f), so it makes nothing now and is no source for gorge's
// trigger-cost gate (rules respay.go resPayNeedsMana over
// windowSourceYields). The dynamic source beside it is folded unbounded only
// while some other source is live; the empty identity source must not be that
// source, or a board of two nothing-making sources over-offers pay.
func TestDynTriggerCostPayIdentityEmptyExtension(t *testing.T) {
	tbl, plains, trig, _, _, _ := dynGapTable(t)
	ident := CardID(len(tbl.Cards))
	dynAny := ident + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Empty Identity", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:         kop.AbMana,
			Cost:         kop.Cost{Flags: kop.CostTap},
			AFlags:       kop.AfManaChoice | kop.AfManaReflect,
			ManaIdentity: true,
			ManaColors:   kop.ColAny,
			ManaN:        1,
		}}},
		kop.CardDef{Name: "Test Dynamic Any", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap},
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrPetal)},
			ManaColors: kop.ColAny,
		}}},
	)
	c6 := manaOf(6, 0)
	ask := func(sc *scen) Decision { return dynPayAsk(t, sc, trig) }

	// precondition: the identity source has a card-level planner unit but its
	// payment view is the no-ability one, and the seat has no commander
	sc := newScenTable(t, tbl, Config{NoElide: true})
	li := sc.perm(ident, 0)
	sc.perm(dynAny, 0)
	dynProbe(t, sc)
	ci := sc.s.cardAt(sc.slot(li))
	if sc.k.cards[ci].manaAb < 0 {
		t.Fatalf("precondition: the identity source has no card-level mana ability (%d)", sc.k.cards[ci].manaAb)
	}
	if pc := sc.s.manaPCGen(sc.k, sc.slot(li)); pc.manaAb >= 0 {
		t.Fatalf("precondition: the empty identity view lists a mana ability (%d)", pc.manaAb)
	}
	if m := sc.s.identityColours(sc.k, 0); m != 0 {
		t.Fatalf("precondition: the seat has a commander identity %#x", m)
	}
	if sc.s.potentialDynOpen(sc.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		sc.s.trigCostPotentialPays(sc.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate opened beside an empty commander-identity source and a zero-amount dynamic source")
	}
	scA := newScenTable(t, tbl, Config{NoElide: true})
	scA.perm(ident, 0)
	scA.perm(dynAny, 0)
	if d := ask(scA); d.Sub != ChGenPay || d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("empty-identity ask %d options %v, want decline only", d.Sub, d.Options[:d.N])
	}

	// control: a live Plains makes the sources flag true and the dynamic
	// source's over-bound fold opens the gate, so the scenarios differ only in
	// the live source the flag reads.
	sc2 := newScenTable(t, tbl, Config{NoElide: true})
	sc2.perm(ident, 0)
	sc2.perm(dynAny, 0)
	sc2.perm(plains, 0)
	dynProbe(t, sc2)
	if !sc2.s.potentialDynOpen(sc2.k, 0, &c6, &payDesc{class: pdOther, subBound: true}) ||
		!sc2.s.trigCostPotentialPays(sc2.k, 0, kop.PayMana, 0, 6, 0, 0) {
		t.Fatal("the gate refused the {6} beside a live source and the zero-amount dynamic source")
	}
	sc2A := newScenTable(t, tbl, Config{NoElide: true})
	sc2A.perm(ident, 0)
	sc2A.perm(dynAny, 0)
	sc2A.perm(plains, 0)
	if d := ask(sc2A); d.Sub != ChGenPay || d.N != 2 ||
		d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("live-source ask %d options %v, want decline then pay", d.Sub, d.Options[:d.N])
	}
}
