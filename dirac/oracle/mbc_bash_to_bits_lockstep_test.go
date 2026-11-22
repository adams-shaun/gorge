package oracle

import "testing"

// TestMBCBashToBitsReprosLockstep replays the recorded game where dirac
// offered a flashback cast of Bash to Bits that gorge did not.
func TestMBCBashToBitsReprosLockstep(t *testing.T) {
	stdReproNamed(t, "bash_to_bits_flashback_refractor")
}
