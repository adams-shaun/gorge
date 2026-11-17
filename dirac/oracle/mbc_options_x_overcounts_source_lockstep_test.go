package oracle

import "testing"

// CR 601.2b/602.2b: the trigger's "you may pay {X}" announces X, and every
// offered value must be payable from the seat's own sources. The seat here has
// one Plains and one Quirion Elves -- a single permanent with two one-mana
// abilities -- so only X = 0..2 is reachable; the gorge offer priced the
// potential pool's per-ability over-bound and offered X = 3 as well.
func TestMBCDecreeOfJusticeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "options_x_overcounts_source")
}
