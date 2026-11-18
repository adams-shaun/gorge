package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The census's lethal-guard read (abPainOf/censusPainDrop): a mana ability
// whose own activation pain — the life its cost pays plus the damage its
// rider deals — reaches the payer's life makes no census unit (gorge's
// planner cuts every plan whose summed pain reaches the life,
// pay/search.go), so the per-tap census must not count its production as
// payable mana on any of availPotentialFor's paths. That is the per-activation
// slice; the plan-summed one (a class capped at (life-1)/pain activations) is
// potUnitsAvail's per-unit cap (TestPainCensusSummedCap), and several
// pain-bearing units sharing the budget keep their individual caps (an
// unmodelled over-bound).

// painOnlyScen builds the pose: six untapped Cabal Pits ({T}: Add {B}. Cabal
// Pit deals 1 damage to you — pain-only, fixed {B}), {C}1 {B}1 floats and
// Caligo Skin-Witch in hand (plain {2}{B}{B}, kicked {4}{B}{B}), at life.
func painOnlyScen(t *testing.T, life int16) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Cabal Pit", Count: 6},
		{Name: "Caligo Skin-Witch", Count: 1},
		{Name: "Plains", Count: 8},
	}
	sc, id := newSetScenAutopay(t, deck)
	for i := 0; i < 6; i++ {
		sc.perm(id["Cabal Pit"], 0)
	}
	sc.resolveAll(t)
	sc.s.pl[0].pool[kop.C] = 1
	sc.s.pl[0].pool[kop.B] = 1
	sc.s.pl[0].life = life
	if sc.s.pl[0].life != life || sc.s.pl[0].pool[kop.B] != 1 || sc.s.pl[0].pool[kop.C] != 1 {
		t.Fatal("precondition: the pool or the life did not take")
	}
	return sc, sc.hand(id["Caligo Skin-Witch"], 0)
}

// TestPainOnlyCensusKickedCast: at life 1 the planner refuses every Cabal Pit
// activation (each deals 1 damage, pain 1 >= life 1), so the census — the
// per-tap view availPotentialFor's walk records, which the kicked arm
// (kickedPotentialPays) and the X-bound sweep price at — holds the floats
// alone ({B}:1 {C}:1), never the six refused {B}, and the kicked {4}{B}{B}
// (which the floats cannot pay) is not offered. At life 7 the planner pays
// the plain cast with the floats and three activations (pain 3 < 7) and the
// kicked one with four (pain 4 < 7), so both are offered: the drop fires only
// at the lethal life. (The plain cast at life 1 is the payment census's own
// gate, availFor/castAffordsR — not this walk's bound.)
func TestPainOnlyCensusKickedCast(t *testing.T) {
	sc, caligo := painOnlyScen(t, 1)
	c := sc.s.objs[caligo].card
	var u [maxPotUnits]potUnit
	spell := payDesc{class: pdSpell, card: c, src: NoSlot, og: sc.s.restrOriginOfObj(caligo)}
	sc.s.availPotentialFor(sc.k, 0, &avail{}, &spell, false, &u)
	if u[0] != (potUnit{mask: 1 << kop.B, n: 1}) || u[1] != (potUnit{mask: 1 << kop.C, n: 1}) || u[2].mask != 0 {
		t.Fatalf("the census holds the activations the lethal guard refuses: units=%v", u[:3])
	}
	if offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is offered on a census that counts activations the lethal guard refuses")
	}
	sc.s.pl[0].life = 7
	if !castModeOffered(sc, caligo, 0) {
		t.Fatal("the plain cast is not offered at life 7")
	}
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is not offered at life 7")
	}
}

// TestAbPainOfSumsLifeAndDamage: a mana ability's activation pain is the life
// its cost pays PLUS the damage its rider deals — gorge's lethal guard sums
// both (pay/search.go), never one or the other.
func TestAbPainOfSumsLifeAndDamage(t *testing.T) {
	if got := abPainOf(nil); got != 0 {
		t.Fatalf("nil: got %d, want 0", got)
	}
	life := kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1}}
	if got := abPainOf(&life); got != 1 {
		t.Fatalf("life only: got %d, want 1", got)
	}
	dmg := kop.Ability{Kind: kop.AbMana, Prog: []kop.Instr{{
		Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.ValueRef{K: kop.VConst, N: 2},
	}}}
	if got := abPainOf(&dmg); got != 2 {
		t.Fatalf("damage only: got %d, want 2", got)
	}
	both := life
	both.Prog = dmg.Prog
	if got := abPainOf(&both); got != 3 {
		t.Fatalf("life + damage: got %d, want 3 (gorge's guard sums them)", got)
	}
}

// TestPainCensusSummedCap: gorge's guard is plan-summed (pay/search.go cuts a
// plan whose summed pain reaches the life and caps a pain-bearing class at
// (life-1-pain)/pain activations), so at life 2 one Cabal Pit (pain 1 < 2) is
// a plan unit but two (summed pain 2) are not. Two untapped Pits and four
// Plains make the kicked Caligo Skin-Witch's {4}{B}{B} payable only off both
// Pits' {B}, which gorge prices no plan for; at life 3 both Pits fit
// (summed pain 2 < 3) and the kick is offered.
func TestPainCensusSummedCap(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Cabal Pit", Count: 2},
		{Name: "Caligo Skin-Witch", Count: 1},
		{Name: "Plains", Count: 8},
	}
	sc, id := newSetScenAutopay(t, deck)
	for i := 0; i < 2; i++ {
		sc.perm(id["Cabal Pit"], 0)
	}
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.resolveAll(t)
	caligo := sc.hand(id["Caligo Skin-Witch"], 0)
	sc.s.pl[0].life = 2
	if sc.s.pl[0].life != 2 || sc.s.pl[0].pool[kop.B] != 0 {
		t.Fatal("precondition: the life did not take or the pool is not empty")
	}
	var u [maxPotUnits]potUnit
	spell := payDesc{class: pdSpell, card: sc.s.objs[caligo].card, src: NoSlot, og: sc.s.restrOriginOfObj(caligo)}
	sc.s.availPotentialFor(sc.k, 0, &avail{}, &spell, false, &u)
	if u[0].mask != 1<<kop.B || u[0].n != 2 || u[0].pain != 1 {
		t.Fatalf("precondition: the census holds both pits as a pain-1 {B} unit: units=%v", u[:3])
	}
	if offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is offered at life 2 on two pain-1 activations whose summed pain reaches the life")
	}
	sc.s.pl[0].life = 3
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is not offered at life 3 (summed pain 2 < 3)")
	}
}
