package oracle

import "testing"

// Sickening Dreams's Discard<X/Card> additional cost: gorge's shape gate
// declines a spell's variable-count discard part (pay.PlanNonManaAdmissible,
// "cost:discard"), so the autopay option list withholds the cast; dirac's
// planDeclinesAddCost admitted the part and offered it beside gorge's two
// options (decision 154 of ev-tail-65-c001, pair 1).
func TestMBCSickeningDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_discardx_addcost")
}

// The same game on the full surface: gorge's potential planner composes the
// announced discard there (pay.DiscardCostPayable never withholds an offer),
// so the option list must keep the cast — gorgeLowers must not decline it
// (an earlier gorgeLowersRider arm did, under-offering by one cast, decision
// 154).
func TestMBCSickeningDreamsFullSurfaceLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_discardx_addcost_full")
}
