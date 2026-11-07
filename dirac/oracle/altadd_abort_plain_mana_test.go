package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Cruel Alliance's either-or additional cost makes the cast unpayable after its
// hybrid pip is announced; the reversal leaves the mana abilities gorge's planner
// had already floated. One of them is a plain source beside a widened restricted
// source tapped for its own census colour (Villainous Hideout's {C} beside Hell's
// Kitchen's {B}), so the {B} stays plain and gorge's next priority still offers
// Cruel Alliance, which no longer is a Villain spell's cost (CR 733.1, 106.6).
func TestAltAddAbortKeepsThePlainSourceManaLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-msh")
	replayRepro(t, reg, set, "alt_add_abort_keeps_plain_source_mana", false, false)
}
