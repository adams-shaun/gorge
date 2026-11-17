package oracle

import "testing"

// Furnace Celebration's "whenever you sacrifice another permanent, you may
// pay {2}" posed the pay option on gorge's side only (decision 590 of 590,
// tail-67-c000 pair 6).
func TestMBCFurnaceCelebrationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "furnace_celebration_sacrifice_trigger_pay")
}
