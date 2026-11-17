package oracle

import (
	"testing"
)

// Emerald Dragonfly's "{G}{G}: gains first strike" is payable from the two
// untapped Eldrazi Spawn tokens (Elven Chorus grants them "{T}: Add one mana
// of any color") while all of the player's Forests are tapped (decision 968 of
// pair 66, pool tail-51, surface autopay): gorge offered pass only; the kernel
// offered the activation. gorge's prefix search branches a tapped Spawn's
// printed sacrifice and spends its 160-node budget before reaching the play;
// the kernel's grantSrch skipped that branch (printedSacVisit) and so reached
// it. Both engines now spend the budget alike and neither offers it.
func TestMBCEmeraldDragonflyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_mana_tap_pays_activation")
}
