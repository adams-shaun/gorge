package oracle

import "testing"

// std-trc's games that once diverged at the end of their script, on the
// autopay surface: both engines now agree there, and for 200 decisions after.
func TestStdTrcReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-trc", []stdRepro{
		{"phased_out_cda_keeps_its_pt", true}, // CR 604.3: a phased-out Lt. Arex's characteristic-defining P/T still sets its base, as gorge's CDASetPT reads it in every zone
	})
}
