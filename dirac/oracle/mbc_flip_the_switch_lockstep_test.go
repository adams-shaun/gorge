package oracle

import "testing"

// Flip the Switch's modes decision: gorge offers mode 1 where dirac offers
// only mode 0 (kfuzz tail-33 pair 1229, decision 241 of the replay).
func TestMBCFlipTheSwitchReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flip_the_switch_modes")
}
