package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The counter-cost mana lockstep: Wall of Roots' "Put a -0/-1
// counter on Wall of Roots: Add {G}. Activate only once each turn." gorge's
// planner census misses it (a relaxable gap: potential_plan.go), so a play
// the lands alone cannot pay is searched for a prefix that activates it
// first (potentialPrefixScript); each counter lowers its toughness, and
// Shock finishes a shrunken Wall.
var kwallrootsDecks = [2][]cardset.Entry{
	{
		{Name: "Wall of Roots", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Centaur Courser", Count: 4},
		{Name: "Craw Wurm", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Runeclaw Bear", Count: 4},
		{Name: "Forest", Count: 18},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Unsummon", Count: 3}, {Name: "Volcanic Hammer", Count: 3},
		{Name: "Mountain", Count: 12}, {Name: "Island", Count: 10},
	},
}

func TestWallrootsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "wallroots", kwallrootsDecks, 1000, "KWALLROOTS_SEEDS")
}

// TestWallrootsLockstepMutation proves the pool pays plays with Wall of
// Roots' mana: without the gap, some seed diverges.
func TestWallrootsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoCtrMana = false }()
	if !lockstepDiverges(t, "wallroots", kwallrootsDecks, 40) {
		t.Fatal("no seed diverged without the counter-cost gap: the pool never needs Wall of Roots' mana")
	}
}
