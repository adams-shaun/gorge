package oracle

import "testing"

// CR 611.2b: Sly Instigator's "can't be blocked" lasts until its controller's
// next turn, so the goaded creature stays unblockable through the opponent's
// following combat. The kernel's unblockable grant ends with the turn, so the
// card is refused rather than offering blocks gorge never poses (decision 289
// of tail-45-c046, Draconic Muralists).
func TestMBCDraconicMuralistsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unblockable_until_next_turn_effect")
}
