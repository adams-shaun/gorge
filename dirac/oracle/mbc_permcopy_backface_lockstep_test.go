package oracle

import "testing"

// CR 707.10f, 707.10g: Tomb of Horrors Adventurer's trigger copies a cast
// Titania, Voice of Gaea (a modal double-faced card cast front face up); the
// copy becomes a token as it resolves. The kernel once stalled `copyface` on
// the copy instead of playing it (tail-76-c000 pair 42, decision 774).
//
// CR 616.1 with CR 616.1g: Rockcaster Platoon's "2 damage to each creature with
// flying and each player" beside Battletide Alchemist's optional "may prevent X
// of that damage to a player" is one event per player, each asked of the
// Alchemist's controller in turn. The kernel dealt the sweep's plain loop past
// the first player's parked ask and stalled (replplus) on the second player's
// (decision 666 of tail-76-c000 pair 28).
func TestMBCTail76ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "permcopy_backface_titania")
	stdReproNamed(t, "optional_prevention_sweeps_each_player_event_by_event")
}
