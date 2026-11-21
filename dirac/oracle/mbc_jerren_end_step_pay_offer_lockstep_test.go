package oracle

import "testing"

// Jerren, Corrupted Bishop's end-step "you may pay {4}{B}{B}" trigger: gorge posed
// only the decline option where the kernel also offered the payment
// (decision 494 of tail-74-c000 pair 12).
func TestMBCJerrenCorruptedBishopReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "jerren_end_step_pay_offer")
}
