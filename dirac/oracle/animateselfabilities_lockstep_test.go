package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Animate Abilities$ on the source itself (kgen animateSelfAbilities): Spawning
// Pool becomes a Skeleton with "{B}: Regenerate this creature" and Lavaclaw
// Reaches an Elemental with "{X}: This creature gets +X/+0" until end of
// turn, the abilities the override record grants the animated land.
var animateSelfAbilitiesDecks = [2][]cardset.Entry{
	{
		{Name: "Spawning Pool", Count: 8}, {Name: "Lavaclaw Reaches", Count: 8}, {Name: "Raging Goblin", Count: 6},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 14}, {Name: "Spawning Pool", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

func TestAnimateAbilitiesDurationLockstep(t *testing.T) {
	t.Parallel()
	lockstepRun(t, "animateselfabilities", animateSelfAbilitiesDecks, 80, true, nil)
}

// TestAnimateAbilitiesDurationLockstepMutation: with the override records'
// grants dropped, some seed diverges (gorge offers the granted ability).
func TestAnimateAbilitiesDurationLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "animateselfabilities", animateSelfAbilitiesDecks, 80, false, func(tbl *kop.Table) {
		ovs := append([]kop.Override(nil), tbl.Overrides...)
		for i := range ovs {
			ovs[i].Grant = [2]uint8{}
		}
		tbl.Overrides = ovs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the animated lands' granted abilities dropped")
	}
}
