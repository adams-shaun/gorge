package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Craft lockstep (CR 702.167a): Clay-Fired Bricks and
// Spring-Loaded Sawblades craft with an artifact (gorge kwCraft: exile
// another artifact you control or an artifact card from your graveyard,
// then the source, and return it transformed: Cosmium Kiln, Bladewheel
// Chariot), with Ornithopters and each other as material, against creature
// beats and burn. The expected answer at every decision is gorge's.
var kcraftDecks = [2][]cardset.Entry{
	{
		{Name: "Clay-Fired Bricks", Count: 6}, {Name: "Spring-Loaded Sawblades", Count: 6}, {Name: "Ornithopter", Count: 6},
		{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 18},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	},
}

func TestCraftLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "craft", kcraftDecks, 1000, "KCRAFT_SEEDS")
}

// TestCraftLockstepMutation proves the pool crafts from the battlefield:
// with the material limited to the graveyard, some seed diverges.
func TestCraftLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "craft", kcraftDecks, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				tbl.Cards[i].Abilities[j].Cost.Flags &^= kop.CostExileCtrlOrGrave
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the battlefield material dropped: the pool never crafts with a permanent")
	}
}
