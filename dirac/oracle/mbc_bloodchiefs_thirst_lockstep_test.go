package oracle

import "testing"

// Bloodchief's Thirst (kicker {2}{B}): gorge offered the kicked cast at priority
// (decision 298, tail-01-c000 pair 43) where the kernel offered none.
func TestMBCBloodchiefSThirstReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bloodchiefs_thirst_kicked_cast_offer")
}
