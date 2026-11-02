package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Starting Town lockstep: {T}: Add {C}, and {T}, Pay 1 life:
// Add one mana of any colour (gorge's last-resort Consequence{Life: 1},
// planned and guarded as a painland's damage), entering tapped unless it
// is its controller's first, second or third turn (ConditionCheckSVar$
// over Count$Compare / YourTurns / HasPropertyActive). Both decks lean on
// the Town for green, white and red. The expected answer at every
// decision is gorge's.
var ktownDecks = [2][]cardset.Entry{
	{
		{Name: "Starting Town", Count: 14}, {Name: "Grizzly Bears", Count: 6}, {Name: "Suntail Hawk", Count: 6},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Starting Town", Count: 12}, {Name: "Llanowar Elves", Count: 6}, {Name: "Shock", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// ktownMixedDecks mixes the Town's paid life with the painlands' damage:
// gorge's planner prices both alike (costPerLifeOrDamage, the summed lethal
// guard), which the kernel's shared pain units follow.
var ktownMixedDecks = [2][]cardset.Entry{
	{
		{Name: "Starting Town", Count: 8}, {Name: "Karplusan Forest", Count: 4}, {Name: "Brushland", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Suntail Hawk", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Starting Town", Count: 8}, {Name: "Battlefield Forge", Count: 6}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Shock", Count: 8}, {Name: "Suntail Hawk", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

func TestTownMixedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "town", ktownMixedDecks, 600, "KTOWN_SEEDS")
}

func TestTownLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "town", ktownDecks, 600, "KTOWN_SEEDS")
}

// TestTownLockstepMutation: with the paid life skipped, some seed diverges.
func TestTownLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPainDamage = true
	defer func() { dirac.DebugNoPainDamage = false }()
	if n := lockstepRun(t, "town", ktownDecks, 200, false, nil); n == 0 {
		t.Fatal("no seed diverged with the paid life skipped")
	}
}

// TestTownLockstepTurnsMutation: with the turn count read as 0 (the Town
// never enters tapped on its controller's turn), some seed diverges.
func TestTownLockstepTurnsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoYourTurns = true
	defer func() { dirac.DebugNoYourTurns = false }()
	if n := lockstepRun(t, "town", ktownDecks, 200, false, nil); n == 0 {
		t.Fatal("no seed diverged with the turn count read as 0")
	}
}
