package oracle

import "testing"

// Boompile's "destroy all nonland permanents" sweep kills p1's three creatures
// and, in the same departure, the two Profane Mementos watching p1's
// graveyard (decision 397 of the recording; tail-18-c000 pair 75). The
// Mementos' printed line is "whenever a creature card is put into an
// opponent's graveyard from anywhere" -- CR 603.6c: an ability that triggers
// when a card is put into a zone "from anywhere" is never treated as a
// leaves-the-battlefield ability, so it has none of CR 603.10a's look-back
// and the kernel must fire it per move, from the listeners still on the
// battlefield as the move completes (gorge's live walk), never with the
// sweep's pre-departure board. The kernel lowered the line's battlefield
// origin onto its Dies event and fired it in the sweep's pre-removal
// fireLeave pass, so the two Mementos still offered a trigger order for the
// third creature's death after they had already left; gorge offered priority.
func TestMBCProfaneMementoReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "anytograve_watcher_destroyed_mid_sweep")
}
