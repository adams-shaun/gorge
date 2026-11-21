package oracle

import "testing"

// CR 601.2f with CR 601.2g: the total cost of a spell is determined at the
// announcement, before its payment, and is paid as determined. Broodstar's
// affinity for artifacts ({1} less per artifact you control) is priced with
// the four artifacts on the board; the payment then sacrifices the Treasure
// token for its mana, which drops the artifact count and would reprice the
// recomputed cost upward. Gorge's witness replay re-asked the offer gate at
// the payment's end and refused the announcement the plan had already priced,
// withholding a cast the kernel offers (decision 280 of 615, tail-12 pair 3,
// seed 283255).
func TestMBCBroodstarReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cost_locked_before_payment_sac")
}
