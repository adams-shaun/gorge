package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Roiling Canopy ("Whenever a Forest you control enters, if you control at least
// five other Forests") with Hexhaven Invigorator's search putting two Forests
// onto the battlefield at once: the six Forests are on the battlefield when the
// enters checks are made (CR 603.2, 603.4, 603.6a), so each of the two sees five
// others and the Canopy triggers twice. The kernel checked the first Forest
// before the second came in and queued one trigger. The fixture is the recorded
// game, played in lockstep: gorge asks p1 to order the two Canopy triggers, and
// the kernel must offer the same two.
func TestRoilingCanopyTwoForestsEnterTogether(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fra")
	replayRepro(t, reg, set, "search_enters_together_canopy_two_forests", true, false)
}
