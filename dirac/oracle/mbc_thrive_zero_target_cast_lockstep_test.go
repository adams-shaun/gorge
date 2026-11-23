package oracle

import "testing"

// CR 601.2c with CR 107.3i: Thrive ({X}{G}) is castable with X = 0 — the
// target slot is "each of X target creatures" (TargetMin$ X TargetMax$ X), so
// zero needs no target, and the cast costs only {G}. Gorge offers the cast;
// the kernel's X ask offered no X at all and dropped the cast from the
// priority options (decision 335 of 335, tail-07-c000 pair 144 seed
// 10041490956089149112).
func TestMBCThriveReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "thrive_zero_target_cast")
}
