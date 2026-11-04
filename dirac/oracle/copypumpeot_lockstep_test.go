package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Gray and Adira, Forever Joined copies a creature its controller controls at
// the beginning of combat after a noncreature spell: the token isn't
// legendary, gains haste only until end of turn (PumpKeywords$ with
// PumpDuration$ EOT, a grant, not a copy exception, CR 707.9b), and is
// sacrificed at the next end step. A copy of the token made by Replicator
// later the same turn has no haste.
func TestCopyPumpEOTLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Gray and Adira, Forever Joined", Count: 8}, {Name: "Replicator", Count: 4},
		{Name: "Planetary Bombardment", Count: 4}, {Name: "Reckless Impulse", Count: 3},
		{Name: "Academy Instructor", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 9},
	}
	b := []cardset.Entry{
		{Name: "Gray and Adira, Forever Joined", Count: 6}, {Name: "Test of Identity", Count: 4},
		{Name: "Boarding Action", Count: 4}, {Name: "Savvy Profiteer", Count: 4},
		{Name: "Island", Count: 11}, {Name: "Mountain", Count: 11},
	}
	fixtureLockstep(t, "std-trk", [2][]cardset.Entry{a, b}, 300)
}
