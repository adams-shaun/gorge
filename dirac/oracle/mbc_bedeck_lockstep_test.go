package oracle

import "testing"

// Bedeck // Bedazzle: dirac offered castface(Bedazzle) at a priority decision
// where gorge offered only pass (decision 455, tail-15-c000 pair 5). Navigator's
// Compass made Jungle Basin a Forest, so tapping it asks a wheel of "Add C G"
// and "Add G"; the lowering could not read the multi-symbol label, aborted, and
// dropped the legal play from gorge's vocabulary as a lower dead end.
func TestMBCBedeckReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_wheel_multi_symbol_label")
}
