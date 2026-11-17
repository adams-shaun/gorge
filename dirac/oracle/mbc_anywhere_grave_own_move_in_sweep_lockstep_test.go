package oracle

import "testing"

// Planar Cleansing destroys all nonland permanents, and Quest for Ancient
// Secrets is among them. Its printed line is "whenever a card is put into your
// graveyard from anywhere" (ChangesZone Origin$ Any, Destination$ Graveyard,
// ValidCard$ Card.!token+YouOwn): CR 603.6c makes that never a
// leaves-the-battlefield ability, so CR 603.10 judges it after the one
// simultaneous departure and CR 113.6 already took the enchanting permanent
// off the battlefield. It must not queue for its own move in the sweep. Gorge
// read the source's LKI in its live walk and queued one (decision 23 of the
// recording: the reference stack held the trigger, the kernel's was empty);
// the browser-side unit test is engine/rules/anywhere_grave_own_move_in_sweep_test.go.
func TestMBCAnywhereGraveOwnMoveSilentInSweepReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "anywhere_grave_own_move_in_sweep")
}
