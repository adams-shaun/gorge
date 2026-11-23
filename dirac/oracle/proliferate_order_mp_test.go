package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// A proliferate that gives a counter to a permanent beside a counter doubler
// (Corpsejack Menace) and a counter plus-N (Hardened Scales) is a CR 616.1
// order competition per kind: gorge poses the replacement ask to the
// recipient's controller while it applies the answered picks
// (effects/counters.go applyProliferate, rules poseAddCounterOrderChoice),
// and CR 616.1e re-poses it over the remainder. dirac applies the picks, in
// answer order, when the proliferate ask closes and parks on the same ask at
// the placement (act_proliferate.go prolifApplyAll, a (pick, kind) cursor in
// res.peekI/res.topN, repl_ctrorder.go ctrOrderPark). The mutation
// (DebugProlifOrderIgnore) ignores the answers during the proliferate.
var prolifOrderPodDeck = []cardset.Entry{
	{Name: "Hardened Scales", Count: 2}, {Name: "Corpsejack Menace", Count: 2},
	{Name: "Steady Progress", Count: 4}, {Name: "Experiment One", Count: 5}, {Name: "Grizzly Bears", Count: 5},
	{Name: "Forest", Count: 12}, {Name: "Island", Count: 5}, {Name: "Swamp", Count: 5},
}

// prolifOrderWatch: gorge asks the CR 616.1 order while a Steady Progress
// resolves.
func prolifOrderWatch(c *PickCtx) bool {
	d := c.G.fr.d
	return d != nil && d.Kind == decision.KReplacement && podOnStack(c.G.E.G, "Steady Progress")
}

// TestFourSeatProliferateCounterOrderLockstep: four-seat games play in
// lockstep through the order ask a proliferate poses.
func TestFourSeatProliferateCounterOrderLockstep(t *testing.T) {
	podClassLockstep(t, prolifOrderPodDeck, 40, 3, prolifOrderWatch, &dirac.DebugProlifOrderIgnore)
}
