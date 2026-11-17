package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 702.51a with CR 106.12: Markov Baron ({2}{B}, Convoke) cast from the
// exile Gonti, Lord of Luxury's rider allows pays {2}{B} with any type of
// mana, but the cost keeps its {B} pip, so a black creature tapped for convoke
// can still pay that pip. gorge offers Gonti a black pick beside its generic
// one; the kernel had folded the pip into generic before the ask and offered
// only the generic pick (decision 445 of dr-mbc-baba pair 136).
func TestMBCGontiLordOfLuxuryReprosLockstep(t *testing.T) {
	t.Parallel()
	replayRepro(t, synthReg(t), cardsettest.Set(t, "mbc"), "convoke_colour_choice_after_cast", true, false)
}
