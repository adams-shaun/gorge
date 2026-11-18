package oracle

import (
	"testing"
)

// CR 305.7: Dream Thrush set Abundant Countryside to a Mountain until end of
// turn, so the land lost its rules-text abilities, its any-colour
// creature-only mana ability with them, and taps for {R} alone. The player's
// other lands (an Island, a Forest, Spire Garden) leave one blue source, so
// Lunar Mystic ({2}{U}{U}) is not castable. The kernel kept Countryside's
// restricted ability beside its intrinsic view and offered the cast
// (decision 229 of 229; tail-28-c000 pair 40).
func TestMBCLunarMysticReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lunar_mystic_cast_offer")
}
