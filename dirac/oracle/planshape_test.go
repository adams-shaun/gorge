package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The autopay surface leaves out a cast whose additional cost is neither a fixed
// sacrifice nor a fixed discard: gorge's planner prices no such cast.
func TestAdditionalCostCastLeavesThePlan(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Fear of Exposure", Count: 8}, {Name: "Abhorrent Oculus", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Thought Scour", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
	}
	lockstepRun(t, "planshape", [2][]cardset.Entry{deck, deck}, 80, true, nil)
}
