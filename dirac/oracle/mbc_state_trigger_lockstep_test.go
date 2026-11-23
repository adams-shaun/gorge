package oracle

import "testing"

// CR 603.8 with CR 113.6: a state trigger (Mode$ Always) is a battlefield-only
// ability and never looks back in time. Lurebound Scarecrow's "when you control
// no permanents of the chosen color, sacrifice it" has no chosen color left once
// the Scarecrow itself has died, so gorge's source-moves look-back queued a
// phantom trigger for the dead Scarecrow beside the Disciple of the Vault pair
// and offered a third trigger_order option the kernel never had (decision 496
// of tail-42-c001 pair 26).
func TestMBCLureboundScarecrowReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "state_trigger_source_leaves_no_lookback")
}
