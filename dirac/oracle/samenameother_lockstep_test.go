package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Targeted.sameName+Other (kgen sameNameT0OtherFilter, kernel sameNameT0
// with PredN 1): Echoing Decay's PumpAll, Homing Lightning's and Izzet
// Staticaster's DamageAll reach every other permanent with the target's name
// and not the target a second time. Both decks are full of same-named
// creatures.
var sameNameOtherDecks = [2][]cardset.Entry{
	{
		{Name: "Echoing Decay", Count: 5}, {Name: "Homing Lightning", Count: 5}, {Name: "Izzet Staticaster", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 16}, {Name: "Raging Goblin", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

func TestFilterHeadSameNameLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "samenameother", sameNameOtherDecks, 80, true, nil)
}

// TestFilterHeadSameNameLockstepMutation: with the target no longer excluded
// (PredN 0), some seed diverges (the target takes the sweep a second time).
func TestFilterHeadSameNameLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "samenameother", sameNameOtherDecks, 80, false, func(tbl *kop.Table) {
		fs := append([]kop.Filter(nil), tbl.Filters...)
		for i := range fs {
			if fs[i].Pred == kop.PredSameNameT0 {
				fs[i].PredN = 0
			}
		}
		tbl.Filters = fs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the target counted among its namesakes")
	}
}
