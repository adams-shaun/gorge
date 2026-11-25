package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The graveyard returns whose Effect gives the entering card extra +1/+1
// counters when it is of a type (Recommission, Heroic Return, What Must Be
// Done, Winter Soldier), played decision by decision against gorge: the
// kernel's two-arm return on the target's type. Mind Stone is the noncreature
// artifact Recommission and What Must Be Done return without the counters.
var kreturnCtrDecks = [2][]cardset.Entry{
	{
		{Name: "Recommission", Count: 4}, {Name: "Heroic Return", Count: 3},
		{Name: "What Must Be Done", Count: 2}, {Name: "Winter Soldier, Reborn Avenger", Count: 3},
		{Name: "Savannah Lions", Count: 6}, {Name: "Mind Stone", Count: 4}, {Name: "Plains", Count: 18},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Shock", Count: 5},
		{Name: "Lightning Bolt", Count: 5}, {Name: "Mountain", Count: 18},
	},
}

func TestReturnCtrLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "returnctr", kreturnCtrDecks, 200, "KRETURNCTR_SEEDS")
}
