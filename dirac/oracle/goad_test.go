package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The goad lockstep (CR 701.15): Taunting Kobold and Goblin
// Racketeer goad on attack, Jeering Homunculus and Insufferable Balladeer as
// they enter (the Homunculus may goad its own side), Disrupt Decorum every
// creature its caster doesn't control. A goaded creature must attack on its
// controller's turn and may not attack the goader's Kaito; the goad ends as
// the goader's next turn begins. The expected answer at every decision is
// gorge's.
var kgoadDecks = [2][]cardset.Entry{
	{
		{Name: "Taunting Kobold", Count: 4}, {Name: "Goblin Racketeer", Count: 4}, {Name: "Jeering Homunculus", Count: 4},
		{Name: "Disrupt Decorum", Count: 3}, {Name: "Insufferable Balladeer", Count: 4}, {Name: "Kaito, Cunning Infiltrator", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Juggernaut", Count: 2}, {Name: "Wall of Wood", Count: 3}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

func TestGoadLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "goad", kgoadDecks, 600, "KGOAD_SEEDS")
}

// TestGoadLockstepMutation proves the pool exercises goad: with every
// goad ignored, some seed diverges.
func TestGoadLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugIgnoreGoad = true
	defer func() { dirac.DebugIgnoreGoad = false }()
	if !lockstepDiverges(t, "goad", kgoadDecks, 40) {
		t.Fatal("no seed diverged with goads ignored: the pool does not exercise them")
	}
}
