package oracle

import "testing"

// TestMBCCryptoplasmReprosLockstep replays the MBC game where Cryptoplasm
// copies Xerex Strobe-Knight in the first upkeep of the turn: the copy gains
// Xerex's "{T}: create a 2/2 Knight, activate only if you've cast two or more
// spells this turn" ability, and its activation gate must survive the copy.
// The recorded game had cast no spell that turn, so the ability is not
// offered; the recording pins the decision where one engine offered it.
func TestMBCCryptoplasmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "clone_copied_activation_gate")
}
