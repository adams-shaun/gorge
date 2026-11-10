package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestWarpHandCastBesideFilteredHandBarredGrant pins the offer's census: a
// hand warp cast may spend only the mana its hand-cast payment admits, so
// Mm'menon's hand-barred granted blue on an artifact is no unit for
// Starwinder's warp {2}{U}{U} even on a table whose live filter makes
// grantCap stand down (CR 106.6, 702.185a). Only Uthros, Titanic Godcore's
// {U} is left, so the warp cast is not offered.
func TestWarpHandCastBesideFilteredHandBarredGrant(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-eoe")
	replayRepro(t, reg, set, "starwinder_warp_needs_second_blue", true, false)
}
