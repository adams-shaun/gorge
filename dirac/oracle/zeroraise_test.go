package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The zeroraise lockstep: Zero, Incorporeal Sensate exiles a nonland
// card from an opponent's hand; for as long as it remains exiled its owner
// may cast it, paying {2} more. The owner is the exiled card's own seat, so
// both decks play Zero and each seat casts its own exiled cards at the
// surcharge. Both decks' creatures, the Aura and the artifact give the
// triggers targets. The expected answer at every decision is gorge's.
var kzeroraiseDecks = [2][]cardset.Entry{
	{
		{Name: "Zero, Incorporeal Sensate", Count: 4}, {Name: "Savannah Lions", Count: 3}, {Name: "Serra Angel", Count: 3},
		{Name: "Pacifism", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Zero, Incorporeal Sensate", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Wind Drake", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Plains", Count: 6},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 2},
	},
}

func TestZeroraiseLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "zeroraise", kzeroraiseDecks, 800, "KZERORAISE_SEEDS")
}

// TestZeroraiseMutation proves the pool casts an exiled card with the
// surcharge: without it, some seed diverges.
func TestZeroraiseMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPartitionRaise = true
	defer func() { dirac.DebugNoPartitionRaise = false }()
	if !lockstepDiverges(t, "zeroraise", kzeroraiseDecks, 200) {
		t.Fatal("no seed diverged without the surcharge: the pool never casts an exiled card at the raise")
	}
}
