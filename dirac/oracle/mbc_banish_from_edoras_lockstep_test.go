package oracle

import "testing"

// Banish from Edoras ({4}{W}, {2} less if it targets a tapped creature): at its
// target ask gorge offered an untapped creature token that dirac withheld
// (decision 614 of tail-32 pair 2).
func TestMBCBanishFromEdorasReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "target_reduce_window_census")
}
