package oracle

import "testing"

// Kethek, Crucible Goliath's end-step dig (FoundDestination$ Battlefield,
// RevealedDestination$ Library, RevealRandomOrder$ True, no
// RevealedLibraryPosition$) puts the revealed rest on the bottom of the
// library in a random order: a battlefield-entering dig is not a
// stay-in-place reveal, so the rest returns to the bottom in the seeded
// random order (7 RNG draws on decision 998 of the tail-23 focus game).
// gorge used to keep the rest on top in reveal order and drew nothing.
func TestMBCKethekTriggerPosReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "diguntil_random_order_rest_bottom_on_entering")
}
