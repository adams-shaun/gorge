package oracle

import "testing"

// Repeal offers an X spell's cast at priority only when X can pay for a legal
// target. This MBC game's priority decision offered gorge the cast where the
// kernel's affordability/target scan offered none; the recording pins the
// priority menu (decision 105 of 105 in the pool's pair 17).
func TestMBCRepealReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "repeal_cast_option")
}
