package oracle

import "testing"

// Invasion of Muraganda (a Siege battle) at decision 653 of tail-22 pair 71: the
// two engines parted at Cards[54].Face after the battle was defeated.
func TestMBCInvasionOfMuragandaReprosLockstep(t *testing.T) {
	t.Parallel()
	corpusDirOrSkip(t) // a clone with no .cards skips before the pool is built
	stdReproNamed(t, "siege_defeated_battle_exile_face")
}
