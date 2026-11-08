package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// A creature returned from the graveyard by its own dies trigger leaves no
// second trigger queued: Burn, Burn, Tree and Fern's chapter I damage killed
// Tom, Bert, and William, whose return resolved, and the next decision sees an
// empty stack in both engines. This is the first divergence of std-hob pair 156
// at mulligans 1 on the full surface.
func TestReturnedCreatureLeavesNoTriggerLockstep(t *testing.T) {
	t.Parallel()
	replayRepro(t, synthReg(t), cardsettest.Synth(t, "std-hob"), "returned_creature_leaves_no_trigger", false, false)
}
