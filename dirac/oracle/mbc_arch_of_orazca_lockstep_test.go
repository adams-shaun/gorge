package oracle

import "testing"

// Invasion of Dominaria's defeat cast beside Arch of Orazca: gorge posed
// choose:defeat_cast_yes/no at decision 677 of the recorded game (tail-63-c001
// pair 60, autopay) where the kernel offered only pass.
func TestMBCArchOfOrazcaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "defeat_cast_offer_needs_payable_cost")
}
