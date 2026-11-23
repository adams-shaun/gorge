package oracle

import "testing"

// Abzan Kin-Guard ({3}{G}) was offered as a cast by gorge but not by dirac at a
// priority decision (decision 163 of tail-55 pair 170).
func TestMBCAbzanKinGuardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "abzan_kin_guard_cast_offer")
}
