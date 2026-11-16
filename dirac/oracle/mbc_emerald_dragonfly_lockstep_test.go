package oracle

import (
	"testing"
)

// Emerald Dragonfly's "{G}{G}: gains first strike" is payable from the two
// untapped Eldrazi Spawn tokens (Elven Chorus grants them "{T}: Add one mana
// of any color") while all of the player's Forests are tapped (decision 968 of
// pair 66, pool tail-51, surface autopay): gorge offered pass only; the kernel
// offered the activation.
func TestMBCEmeraldDragonflyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_mana_tap_pays_activation")
}
