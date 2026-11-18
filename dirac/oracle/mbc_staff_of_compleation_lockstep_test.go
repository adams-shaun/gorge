package oracle

import "testing"

// CR 605.3b, 602.2: a {T}, pay-life mana source pays for another ability's
// generic mana like any mana ability (Staff of Compleation's own {5} untap, and
// Witherbloom Campus's {4},{T} scry beside it). The kernel offered both with
// four lands and the Staff untapped; gorge offered neither (decision 588 of
// the evidence loop's tail-66-c000 pair 84).
func TestMBCStaffOfCompleationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pay_life_mana_source_beside_own_mana_cost")
}
