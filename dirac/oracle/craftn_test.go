package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The multi-card Craft lockstep (CR 702.167a): Visage of Dread crafts by
// exiling two other creatures from the battlefield and/or the graveyard
// (gorge kwCraft: ExileCtrlOrGrave<2/Creature.Other>, then the source),
// against burn that fills the graveyard. The expected answer at every
// decision is gorge's.
var kcraftnDecks = [2][]cardset.Entry{
	{
		{Name: "Visage of Dread", Count: 6}, {Name: "Walking Corpse", Count: 18}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
}

func TestCraftNLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "craftn", kcraftnDecks, 300, "KCRAFTN_SEEDS")
}

// TestCraftNLockstepMutation proves the pool pays a two-card craft: with the
// material count cut to one, some seed diverges.
func TestCraftNLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "craftn", kcraftnDecks, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if c := &tbl.Cards[i].Abilities[j].Cost; c.Flags&kop.CostExileGrave != 0 && c.N == 2 {
					c.N = 1
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with the material cut to one card: the pool never crafts Visage of Dread")
	}
}
