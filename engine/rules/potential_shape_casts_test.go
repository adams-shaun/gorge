package rules

import "testing"

// TestPotentialShapeCastsNameAnnouncedCosts: the payment build's shape census
// (decision.PotentialShapeCasts) names a hand cast the planner declines for
// its cost's shape -- an {X} announced as it is cast (CR 107.3, 601.2b) -- and
// not a plain cost the planner prices.
func TestPotentialShapeCastsNameAnnouncedCosts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cost  string
		shape bool
	}{{"X R", true}, {"R", false}} {
		e, _, spell := newFixtureDeck(t, 9415, "Name:Census Spark\nManaCost:"+tc.cost+"\nTypes:Instant\nA:SP$ Draw | NumCards$ 1\nOracle:x\n")
		onBoard(t, e, 0, "Name:Census Mountain\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ This land taps for one red.\nOracle:x\n")
		d := ppPriority(t, e)
		e.EnsurePaymentActions()
		if !d.ShapeCastsKnown {
			t.Fatalf("%s: shape census not built", tc.cost)
		}
		if got := containsObjID(d.PotentialShapeCasts, spell); got != tc.shape {
			t.Errorf("%s: in PotentialShapeCasts = %v, want %v (census %v)", tc.cost, got, tc.shape, d.PotentialShapeCasts)
		}
	}
}
