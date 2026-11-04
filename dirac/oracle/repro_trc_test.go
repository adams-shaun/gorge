package oracle

import "testing"

// std-trc's games that once diverged at the end of their script, on the
// autopay surface: both engines now agree there, and for 200 decisions after.
func TestStdTrcReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trc", []stdRepro{
		{"phased_out_cda_keeps_its_pt", true, false},                  // CR 604.3: a phased-out Lt. Arex's characteristic-defining P/T still sets its base, as gorge's CDASetPT reads it in every zone
		{"reg_barclay_phased_out_copy_keeps_exceptions", true, false}, // CR 702.26b: a phased-out copy keeps its layer-1 copy exceptions
	})
}
