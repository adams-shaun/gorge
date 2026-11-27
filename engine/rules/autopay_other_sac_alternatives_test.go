package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A cast whose own cost sacrifices a creature keeps its one plan: the searched
// alternatives (EnsurePaymentPlanAlternatives) never name a creature the cost
// still needs.

const (
	otherSacFlingThreeSrc = "Name:OtherSac Fling Three\nManaCost:3\nTypes:Instant\nA:SP$ Draw | Cost$ 3 Sac<1/Creature> | NumCards$ 1\nOracle:x\n"
	otherSacColourlessLd  = "Name:OtherSac Wastes\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ x\nOracle:x\n"
)

// TestAlternativePlansLeaveAdditionalCostVictim: a colourless land and two
// altars fund the cost three ways, but a plan that runs both altars sacrifices
// both creatures and strands the cost's own sacrifice.
func TestAlternativePlansLeaveAdditionalCostVictim(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 4715, otherSacFlingThreeSrc)
	onBoard(t, e, 0, otherSacColourlessLd)
	onBoard(t, e, 0, otherSacAltarSrc)
	onBoard(t, e, 0, otherSacAltarSrc)
	c1 := onBoard(t, e, 0, otherSacCreatureSrc)
	c2 := onBoard(t, e, 0, otherSacCreatureSrc)
	d := ppPriority(t, e)
	found := false
	for _, a := range e.EnsurePaymentPlanAlternatives() {
		if a.Cast.Object != spell {
			continue
		}
		found = true
		if len(a.Plans) == 0 {
			t.Fatalf("precondition: no plan offered for the spell")
		}
		for i, pl := range a.Plans {
			victims := map[state.ObjID]bool{}
			for _, act := range pl.Activations {
				for _, v := range act.Sacrifices {
					victims[v] = true
				}
			}
			if victims[c1] && victims[c2] {
				t.Fatalf("plan %d (of %d) sacrifices both creatures, leaving none for the cost: %+v", i, len(a.Plans), pl.Activations)
			}
		}
	}
	if !found || d == nil {
		t.Fatalf("precondition: the spell has no payment action")
	}
}
