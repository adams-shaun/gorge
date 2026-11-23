package oracle

import "testing"

// CR 603.10a with CR 700.14-style batching: False Memories' delayed trigger
// exiles seven cards from its controller's graveyard as one hidden zone move
// (a ChangeZone Hidden pick), so Undead Hand Ninja's "whenever one or more
// creature cards leave your graveyard" fires once for the five creature cards.
// The kernel opened no zone batch for the pick: it queued one trigger per card
// and posed a trigger-order decision where gorge had one trigger on the stack
// (decision 328 of tail-20-c000 pair 96, projection NStack).
func TestMBCHeWhoHungersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hidden_graveyard_exile_is_one_leave_batch")
}
