package oracle

import "testing"

// Polymorph: the destroyed creature's controller reveals cards from the top of
// their library until a creature card, puts it onto the battlefield and shuffles
// the rest in. The two engines parted on how many RNG draws the resolution
// consumed (tail-59 pair 53, seed 1234: ref 99, kernel 103).
func TestMBCPolymorphReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "polymorph_reveal_shuffle")
}
