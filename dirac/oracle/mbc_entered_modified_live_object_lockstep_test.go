package oracle

import (
	"testing"
)

// CR 702.36 with CR 400.7: "a modified creature died under your control this
// turn" reads the creature as it last existed on the battlefield -- Forge
// keeps the moved card's latest state with each zone entry (GameAction's
// lastKnownInfo, "the modified state of the card is also reported here"), so
// a Glen Elendra Archmage that died unmodified and was returned by persist
// with the -1/-1 counter was still not a modified creature when it died, and
// Intermediate Chirography's level-3 end-step condition is false. gorge's
// matchesEnteredEntry matched the battlefield departure against the object
// where it now lives, so the re-entry counter made the condition true and
// gorge posed a trigger_order decision over two Chirography triggers the
// kernel never queued (decision 774 of 774; pool tail-13, pair 7, surface
// autopay, seed 13359707668992827353).
func TestMBCTail13ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "entered_modified_live_object")
}
