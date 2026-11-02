package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Jodah lockstep: whenever its controller casts a legendary
// spell from hand, Jodah, the Unifier exiles cards from the top of the
// library until a legendary nonland card with lesser mana value, offers it
// free, and puts the rest on the bottom in a random order (gorge
// ChangeZoneAll RandomOrder$ through the engine rng). Vanilla legends of
// every mana value feed both the trigger and the dig.
var kjodahDecks = [2][]cardset.Entry{
	{
		{Name: "Jodah, the Unifier", Count: 4}, {Name: "Birds of Paradise", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Tyrox, Saurid Tyrant", Count: 2},
		{Name: "Kalakscion, Hunger Tyrant", Count: 2}, {Name: "Jasmine Boreal", Count: 2},
		{Name: "Tobias Andrion", Count: 2}, {Name: "Jedit Ojanen", Count: 1}, {Name: "Lady Orca", Count: 1},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Unsummon", Count: 3}, {Name: "Counterspell", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Island", Count: 11},
	},
}

func TestJodahLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "jodah", kjodahDecks, 1000, "KJODAH_SEEDS")
}

// TestJodahLockstepMutation proves the pool digs: without the random
// order, some seed diverges.
func TestJodahLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoDugShuffle = true
	defer func() { dirac.DebugNoDugShuffle = false }()
	if !lockstepDiverges(t, "jodah", kjodahDecks, 200) {
		t.Fatal("no seed diverged without the random order: Jodah never digs past one card")
	}
}
