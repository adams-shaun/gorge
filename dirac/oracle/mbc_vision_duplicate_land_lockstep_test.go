package oracle

import "testing"

// TestMBCVisionDuplicateLandReprosLockstep: a land in hand is one play, not
// two, while Vision, Spectral Synthezoid's free noncreature may-play static
// covers it (CR 305.1: a land is played, never cast; the offer builder lists
// one land play per object). The static's permission reaches a land in hand
// (a land is a noncreature card), so without the one-play dedupe the priority
// menu listed the land twice beside Vision.
func TestMBCVisionDuplicateLandReprosLockstep(t *testing.T) {
	stdReproNamed(t, "vision_duplicate_hand_land")
}
