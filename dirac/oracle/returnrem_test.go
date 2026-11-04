package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A blink of two objects returns both (CR 608.2c), each in its turn, a clone's
// copy ask suspending the walk over the rest.
func TestReturnEveryRememberedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Displace", Count: 5}, {Name: "Ghostly Flicker", Count: 5}, {Name: "Lake-town Mariners", Count: 2},
		{Name: "Clone", Count: 5}, {Name: "Elvish Visionary", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 6},
	}
	lockstepRun(t, "returnrem", [2][]cardset.Entry{deck, deck}, 120, true, nil)
}

// A land returned together with Attercop is a land entering while Attercop is
// there, whichever came back first (CR 603.6a, 603.10).
func TestReturnedTogetherSeeEachOther(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Lake-town Mariners", Count: 14}, {Name: "Attercop", Count: 8},
		{Name: "Forest", Count: 10}, {Name: "Island", Count: 8},
	}
	lockstepRun(t, "returnrem", [2][]cardset.Entry{deck, deck}, 120, true, nil)
}
