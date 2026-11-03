package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Instants and sorceries granted storm (Prismari, the Inspiration; the
// emblem Ral, Crackling Wit's ultimate makes): the cast trigger copies the
// spell once per spell cast before it this turn, by any player, and the copy's
// controller may choose new targets (CR 702.40a, 707.10c). Without the trigger
// a game with a Prismari in play diverges at the first stacked spell.
var stormGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Prismari, the Inspiration", Count: 4}, {Name: "Ral, Crackling Wit", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Brainstorm", Count: 3},
		{Name: "Impulse", Count: 2}, {Name: "Divination", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Brainstorm", Count: 4},
		{Name: "Impulse", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

func TestStormGrantLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stormgrant", stormGrantDecks, 64, "KSTORMGRANT_SEEDS")
	lockstepSet(t, "stormgrant", [2][]cardset.Entry{stormGrantDecks[1], stormGrantDecks[0]}, 64, "KSTORMGRANT_SEEDS")
}

// TestStormGrantMutation proves the lockstep reads the native storm trigger.
func TestStormGrantMutation(t *testing.T) {
	dirac.DebugIgnoreStorm = true
	defer func() { dirac.DebugIgnoreStorm = false }()
	if !lockstepDiverges(t, "stormgrant", stormGrantDecks, 64) {
		t.Fatal("ignoring the storm trigger left the lockstep clean")
	}
}
