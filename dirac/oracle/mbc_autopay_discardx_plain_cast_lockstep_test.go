package oracle

import (
	"testing"
)

// Nostalgic Dreams ("As an additional cost to cast this spell, discard X
// cards. Return X target cards from your graveyard to your hand. Exile
// Nostalgic Dreams.") in a hand whose floating pool is empty: gorge's
// payment planner never prices an announced Discard<X/...> additional cost
// (pay.PlanNonManaAdmissible declines every variable-count discard, rules
// payment_plan.go paymentPlanCastShapeDetailAltAdd), so the autopay surface
// drops the plain cast it reaches only through the potential walk
// (Config.GorgeAutopaySurface). The kernel's model of that decline
// (planDeclinesAddCost) admitted CostDiscard at any count and offered the
// cast from the potential walk (decision 151 of pair 5, pool tail-42,
// surface autopay, mulligans 0, seed 779964; class options options).
func TestMBCNostalgicDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "autopay_discardx_plain_cast")
}
