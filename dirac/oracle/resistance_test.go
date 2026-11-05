package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Resistance Is Futile lockstep: its second mode, "return target artifact
// creature card with mana value 3 or less from your graveyard to the
// battlefield" (CR 202.3d/e, 608.2b), against artifact creatures it may return
// (Collective Drone and Tenacious Drone at mana value 2, Automated Warfare
// System at 3) and ones it may not (Assault Drone at 4, Hive Mind Coprocessor
// at 5) beside a nonartifact creature it may not return at all. The expected
// answer at every decision is gorge's.
var kresistanceDecks = [2][]cardset.Entry{
	{
		{Name: "Resistance Is Futile", Count: 4}, {Name: "Collective Drone", Count: 4},
		{Name: "Tenacious Drone", Count: 4}, {Name: "Automated Warfare System", Count: 3},
		{Name: "Assault Drone", Count: 3}, {Name: "Hive Mind Coprocessor", Count: 2},
		{Name: "D'Ghor Bloodletter", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 2},
	},
}

func TestResistanceIsFutileLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "resistance", kresistanceDecks, 300, "KRESISTANCE_SEEDS")
}

// TestResistanceIsFutileLockstepMutation proves the pool exercises the mana
// value bound: with every PredTypesAllCMCLE rewritten to a plain PredTypesAll
// (the bound dropped), some seed diverges because the kernel offers an
// artifact creature of mana value 4 or more that gorge refuses.
func TestResistanceIsFutileLockstepMutation(t *testing.T) {
	n := lockstepRun(t, "resistance", kresistanceDecks, 120, false, func(tbl *kop.Table) {
		filters := append([]kop.Filter(nil), tbl.Filters...)
		for i := range filters {
			if filters[i].Pred == kop.PredTypesAllCMCLE {
				filters[i].Pred = kop.PredTypesAll
			}
		}
		tbl.Filters = filters
	})
	if n == 0 {
		t.Fatal("no seed diverged with the mana-value bound dropped: the pool does not reach the second mode with an artifact creature of mana value 4 or more in the graveyard")
	}
}
