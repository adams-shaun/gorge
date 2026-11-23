package oracle

import "testing"

// CR 616.1e with CR 615.7: Indestructible Aura's "prevent all damage" shield
// sits on Guard Gomazoa beside the creature's own printed "prevent all combat
// damage that would be dealt to CARDNAME". Both are bare full preventions, so
// whichever applies first prevents the combat damage whole and neither is
// spent; gorge applies one without the CR 616.1 order ask, and the kernel must
// not stall at prevnext (tail-37-c001 pair 26, decision 317).
func TestMBCTail37ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevnext")
	stdReproNamed(t, "prevnext_gated_plus_host")
}
