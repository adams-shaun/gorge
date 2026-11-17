package oracle

import "testing"

// CR 603.2 with Forge's FirstTime$ True on Mode$ LifeLost: Intermediate
// Chirography's level-2 trigger ("whenever you lose life for the first time
// each turn") fires once for a turn's first loss. The kernel checked the
// first-time gate on life gain only, so a combat hit queued the trigger twice
// and dirac posed a trigger-order decision gorge never did. The evidence loop
// named Llanowar Loamspeaker, the latest action's source; the duplicated
// trigger is Intermediate Chirography's. The tail-04 pool was re-dealt before
// this landed, so the recording is the same decision-kind divergence on the
// current tail-13 pool, replayed on its own decks' table.
func TestMBCLlanowarLoamspeakerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "llanowar_loamspeaker")
}
