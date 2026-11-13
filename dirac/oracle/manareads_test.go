package oracle

import "testing"

// With a census no relaxed proof can run on, a hybrid cast is one of the four
// plays the planner searches, ahead of the battlefield's activations, so an
// activation past the fourth play is not offered.
func TestHybridCastTakesASearchSlotWhenUnrelaxable(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-spm", Config{GorgeAutopaySurface: true}, hybridCast)
}

// A plain cast beside a free one is a play of its own: gorge's potential planner
// prices the printed cost and leaves only the free cast unpriced, so with a
// mana-spent reader out (the cast planner declines every cast) the plain cast of
// Prismari Charm is offered beside Zaffai and the Tempests' free one.
func TestPlainCastBesideAFreeCastIsPricedLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "plain_cast_beside_free_cast_priced", "std-sos", Config{})
}
