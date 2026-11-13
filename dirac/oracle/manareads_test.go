package oracle

import "testing"

// A plain cast beside a free one is a play of its own: gorge's potential planner
// prices the printed cost and leaves only the free cast unpriced, so with a
// mana-spent reader out (the cast planner declines every cast) the plain cast of
// Prismari Charm is offered beside Zaffai and the Tempests' free one.
func TestPlainCastBesideAFreeCastIsPricedLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "plain_cast_beside_free_cast_priced", "std-sos", Config{})
}
