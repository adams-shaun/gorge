package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MinMana$ floor on "activated abilities cost {N} less"
// (gorge pay.ApplyTo's per-reduction Floor, which caps
// the reduction and never raises a cost). Deck A: Forensic Gadgeteer ({1}
// floor, Amount 1) with Mutagen Man's unfloored reduction; a Clue ({2}) costs
// {1} under one Gadgeteer and stays {1} under two. Deck B: Agatha of the
// Vile Cauldron (Amount$ X = her power, floor {1}) over Exhaust creatures.
var kreduceminmanaDecks = [2][]cardset.Entry{
	{
		{Name: "Forensic Gadgeteer", Count: 4}, {Name: "Mutagen Man, Living Ooze", Count: 2},
		{Name: "Knowledge Seeker", Count: 4}, {Name: "Keen Buccaneer", Count: 3},
		{Name: "Forecasting Fortune Teller", Count: 3},
		{Name: "Island", Count: 22}, {Name: "Mutavault", Count: 1}, {Name: "Mishra's Foundry", Count: 1},
	},
	{
		{Name: "Agatha of the Vile Cauldron", Count: 4}, {Name: "Boom Scholar", Count: 3},
		{Name: "Greasewrench Goblin", Count: 3}, {Name: "Stampeding Scurryfoot", Count: 3},
		{Name: "Camera Launcher", Count: 3},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
		{Name: "Treetop Village", Count: 1}, {Name: "Rogue's Passage", Count: 1},
	},
}

func TestReduceminmanaLockstep(t *testing.T) {
	t.Parallel()
	before, raised := dirac.AbReducedPriced.Load(), dirac.AbFloorRaised.Load()
	lockstepSet(t, "reduceab", kreduceminmanaDecks, 128, "KREDUCEMINMANA_SEEDS")
	n, r := dirac.AbReducedPriced.Load()-before, dirac.AbFloorRaised.Load()-raised
	t.Logf("%d reduced ability pricings, %d where the MinMana$ floor raised the generic", n, r)
	if n < 40 {
		t.Fatalf("only %d reduced ability pricings: the pool does not exercise the static", n)
	}
	if r < 5 {
		t.Fatalf("only %d floor raises: the pool never meets the floor", r)
	}
}

func TestReduceminmanaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoMinManaFloor = true
	defer func() { dirac.DebugNoMinManaFloor = false }()
	if !lockstepDiverges(t, "reduceab", kreduceminmanaDecks, 128) {
		t.Fatal("no seed diverged with the MinMana$ floor dropped: the pool never meets the floor")
	}
}
