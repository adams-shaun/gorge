package oracle

import "testing"

// CR 702.37c: a card with morph may be cast face down for {3}. gorge offered
// castface for Krosan Colossus at priority (decision 520 of tail-17 pair 18)
// where the kernel did not.
func TestMBCKrosanColossusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "morph_face_down_cast_offered")
}
