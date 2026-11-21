package oracle

import "testing"

// TestMBCVaultOfTheArchangelModesReprosLockstep replays the recorded tail-34 game
// (pair 50) in which gorge and the kernel part at a modes decision for p1
// after Vault of the Archangel's activation: gorge offers mode 1, the kernel
// does not.
func TestMBCVaultOfTheArchangelModesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "vault_of_the_archangel_modes")
}
