package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The scoped-prevention lockstep: effects that prevent all damage dealt this
// turn to their controller and creatures they control, to creatures only, or
// by creature sources only (an Effect's bodyless DamageDone Prevent$
// replacement in the Command zone, CR 615.1), against red burn and
// creatures. The expected answer at every decision is gorge's.
var kprevscopeDecks = [2][]cardset.Entry{
	{
		{Name: "Eerie Interference", Count: 6}, {Name: "The Needs of the Many", Count: 4},
		{Name: "Crystal Fragments", Count: 3}, {Name: "Savannah Lions", Count: 8}, {Name: "Serra Angel", Count: 4},
		{Name: "Plains", Count: 15},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestPrevScopeLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "prevscope", kprevscopeDecks, 1000, "KPREVSCOPE_SEEDS")
}

// TestPrevScopeLockstepMutation proves the pool prevents damage: with the
// scoped prevention dropped (the Effect still resolving), some seed diverges.
func TestPrevScopeLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevScope = true
	defer func() { dirac.DebugNoPrevScope = false }()
	if !lockstepDiverges(t, "prevscope", kprevscopeDecks, 200) {
		t.Fatal("no seed diverged with the scoped prevention dropped: the pool never has damage prevented to a player or its creatures")
	}
}
