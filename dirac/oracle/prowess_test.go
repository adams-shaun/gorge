package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted-prowess lockstep: Bria ("other creatures you control have
// prowess") and Sokka ("other Allies you control have menace and prowess")
// on one battlefield. Sokka's Ally tokens are granted prowess twice, Sokka
// and Heartfire Immolator print it and get one granted trigger, and the
// eight Shocks are the noncreature spells that fire them all (CR 702.108b).
var kprowessDeck = []cardset.Entry{
	{Name: "Bria, Riptide Rogue", Count: 4}, {Name: "Sokka, Tenacious Tactician", Count: 4},
	{Name: "Heartfire Immolator", Count: 4}, {Name: "Shock", Count: 8},
	{Name: "Island", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 8},
}

var kprowessDecks = [2][]cardset.Entry{kprowessDeck, kprowessDeck}

func TestProwessLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "prowess", kprowessDecks, 600, "KPROWESS_SEEDS")
}

// TestProwessLockstepMutation proves the pool reaches a recipient with two
// grants: with each permanent's granted prowess capped at one, some seed
// diverges.
func TestProwessLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugProwessOnce = true
	defer func() { dirac.DebugProwessOnce = false }()
	if lockstepDiverges(t, "prowess", kprowessDecks, 200) {
		return
	}
	t.Fatal("no seed diverged with granted prowess capped at one: the pool did not reach a 2-grant recipient")
}
