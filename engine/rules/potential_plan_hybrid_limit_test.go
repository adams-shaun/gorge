package rules

import "testing"

// TestPotentialHybridPlanKeepsBudgetLimitedWitness: a cost with a hybrid pip is
// planned by resolving the pip to each colour, and a resolution whose search
// stops at the node budget still holds a plan that pays. Twenty red sources that
// differ only in ability identity exhaust the budget (the search cannot prove
// which ten are best), and the spell, nine generic and one {R/G}, is payable
// from them: the potential plan is a witness, not an unsupported hybrid cost.
func TestPotentialHybridPlanKeepsBudgetLimitedWitness(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9996, srchSpell("9 RG"))
	for k := 0; k < 20; k++ {
		onBoard(t, e, 0, srchDistinctRock(k))
	}
	toMain1(t, e)
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), spell, "")
	if pp.Reason != "" || pp.Plan == nil {
		t.Fatalf("potential cast of the hybrid spell: reason %q detail %q plan %v, want a witness", pp.Reason, pp.Detail, pp.Plan)
	}
	if n := len(pp.Plan.Activations); n != 10 {
		t.Fatalf("witness taps %d sources, want 10", n)
	}
}
