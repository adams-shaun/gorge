package oracle

import "testing"

// CR 733.2: a card whose cast was reversed twice in a row with no progress is
// held out of the priority options as a whole, so its hand-zone activated
// ability (Rebuild's cycling, the {U}{W} floating after the undone casts would
// pay it) is withheld too, as the battlefield and graveyard offers already
// did. The kernel's hand-ability walk offered it (tail-53 pair 5, decision
// 478).
func TestMBCRebuildReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hand_activation_held_out_after_second_abort")
}
