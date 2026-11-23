package oracle

import "testing"

// CR 305.7: Blood Moon makes every nonbasic land a Mountain and takes the
// land's own abilities with it as part of that layer-4 effect, so Sand
// Silos' storage-counter tap -- a dynamic amount whose cost the planner
// cannot run, hence no planner unit (manaAb and vi both -1, its production
// colours on manaCol) -- is gone and the land keeps only the intrinsic
// "{T}: Add {R}" the basic land type gives it. manaPCGen ran its loss
// check only over the cards holding a planner mana ability or a
// dynamic-amount view, so the kernel answered the deferred source's view
// with the card's own facts and blgView stalled landgrant on the granted
// Mountain colour it does not make (evloop tail-23 c000, pair 6).
func TestMBCTail23ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "landgrant_setter_gap")
}
