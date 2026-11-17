package oracle

import "testing"

// CR 118.12 with CR 608.2c: Withdraw's second bounce ("return another target
// creature to its owner's hand unless its controller pays {1}") asks the
// controller of the creature that second leg targets, not the controller of
// the first leg's target (tail-08 pair 18, autopay: the caster's own creature
// went first, the opponent's second, and the unless-pay question belongs to
// the opponent).
func TestMBCWithdrawReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_payer_second_target_controller")
}
