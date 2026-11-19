package pay

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// A hybrid pip under an AnyColor may-play rider is funded by one COLOURED unit
// of any colour ("any color" never includes colourless, CR 107.4c), so a pin
// must hold it coloured with the plain coloured pips: the folded plan may tap
// a colourless-only source, but the real payment then cannot settle the pip.
func TestPlanManaUnderRiderPinHoldsHybridPipsColoured(t *testing.T) {
	t.Parallel()
	hyb := cost.Cost{Hybrid: []cost.ManaPair{{A: 'W', B: 'U'}, {A: 'W', B: 'U'}}}
	for pin := state.MW; pin <= state.MG; pin++ {
		got := PlanManaUnderRiderPinned(hyb, PipRider{AnyColor: true}, int(pin))
		if got.Colored[pin] != 2 || got.Generic != 0 || len(got.Hybrid) != 0 {
			t.Fatalf("pin %c: generic %d, coloured %v, hybrid %v, want the two pips on the pin and none generic",
				"WUBRG"[pin], got.Generic, got.Colored, got.Hybrid)
		}
	}
	// An AnyType rider lets colourless mana pay, so it pins nothing.
	if got := PlanManaUnderRiderPinned(hyb, PipRider{AnyColor: true, AnyType: true}, int(state.MB)); got.Colored[int(state.MB)] != 0 {
		t.Fatal("an AnyType rider pinned a hybrid pip")
	}
	// The pinned alternatives list carries the hybrid-only cost's five pins.
	_, n := PlanManaUnderRiderAlternatives(hyb, PipRider{AnyColor: true})
	if n != 7 { // folded + five pins + the unfolded cost (its wire differs)
		t.Fatalf("alternatives for a hybrid-only cost: %d, want folded + five pins + unfolded", n)
	}
}

// A plan built for a board-spread pin (the folded cost's pinned units
// re-coloured across the WUBRG slots) matches by shape: the wire names the
// distribution, the matched cost is the re-coloured fold, and a wire that
// over-asks one colour or carries an extra {C} is not the spread.
func TestPlanManaUnderRiderMatchingRebuildsTheSpreadShape(t *testing.T) {
	t.Parallel()
	hyb := cost.Cost{Hybrid: []cost.ManaPair{{A: 'W', B: 'U'}, {A: 'W', B: 'U'}, {A: 'W', B: 'U'}, {A: 'W', B: 'U'}}}
	rider := PipRider{AnyColor: true}
	spread := PlanManaUnderRider(hyb, rider) // generic 4
	spread.Generic -= 4
	spread.Colored[int(state.MB)] = 3
	spread.Colored[int(state.MR)] = 1
	wire := WireCost(spread)
	got, ok := PlanManaUnderRiderMatching(hyb, rider, wire)
	if !ok || got.Colored[int(state.MB)] != 3 || got.Colored[int(state.MR)] != 1 || got.Generic != 0 {
		t.Fatalf("spread wire %+v: matched %+v ok=%v, want the B3 R1 re-coloured fold", wire, got, ok)
	}
	// A wire over-asking one colour is a single-colour pin, matched exactly.
	pin := PlanManaUnderRiderPinned(hyb, rider, int(state.MU))
	got, ok = PlanManaUnderRiderMatching(hyb, rider, WireCost(pin))
	if !ok || got.Colored[int(state.MU)] != 4 {
		t.Fatalf("pin wire: matched %+v ok=%v, want the U pin", got, ok)
	}
	// A wire whose pinned sum is not the pip count, or that drags an extra
	// {C} in, matches nothing.
	bad := WireCost(spread)
	bad.Mana[state.MB] = 2
	if _, ok := PlanManaUnderRiderMatching(hyb, rider, bad); ok {
		t.Fatal("a wire pinning two units matched a four-pip cost")
	}
	wrongC := wire
	wrongC.Mana[state.MC] = 1
	if _, ok := PlanManaUnderRiderMatching(hyb, rider, wrongC); ok {
		t.Fatal("a wire with a {C} unit matched a {C}-free cost")
	}
	// Without an AnyColor rider the spread shape admits nothing.
	if _, ok := PlanManaUnderRiderMatching(hyb, PipRider{}, wire); ok {
		t.Fatal("the spread shape matched without the rider")
	}
}
