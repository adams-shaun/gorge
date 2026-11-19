package oracle

import "testing"

// Madness (CR 702.35b): as the cast-or-graveyard trigger of an exiled card
// resolves, "cast it for its madness cost" is offered whenever the cast is
// payable. Burning Earth on the battlefield is a Taps watcher whose tap the
// payment planner defers, so the two nonbasic lands beside it (Scrying
// Sheets, Snowfield Sinkhole) hold the {C} the cost's generic part needs --
// gorge's offer gate prices the cast against its potential pool, which
// counts a census-deferred source, and its census cannot prove the cast
// unpayable. The kernel's madness gate read only its census and window,
// which both leave the deferred source out, and offered only "put it into
// the graveyard" (decision 642 of 642; tail-71 pair 8, seed 973599).
func TestMBCRevolutionistReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_tapsdefer_cast_offer")
}
