package oracle

import "testing"

// dr-mbc-baaa pair 8676 (seed 12013510227201229287), shrunk to 183 actions: at a
// main-phase priority decision the kernel offers p1's Imodane's Recruiter its
// Adventure face -- castface(#49,-,18), the Train Troops (4W) adventure_alt cast
// from the hand -- beside the ordinary creature cast both engines offer, and the
// reference (gorge) withholds the face route. One engine offering an Adventure
// cast the other does not is the `options options` lockstep class; the two
// offer gates (kernel face_castface.go faceCastable, gorge rules legal_walk
// offerCastableAsFace under the potential planner) must agree.
func TestMBCImodaneSRecruiterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "imodanes_recruiter_adventure_alt")
}
