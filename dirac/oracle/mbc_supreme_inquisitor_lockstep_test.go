package oracle

import "testing"

// Supreme Inquisitor's "tap five untapped Wizards" ability: gorge and dirac
// offered different modes at a modes decision (decision 336, tail-73 pair 20).
func TestMBCSupremeInquisitorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "supreme_inquisitor_tap_wizards_modes")
}
