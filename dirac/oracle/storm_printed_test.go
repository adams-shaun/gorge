package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Printed K:Storm (CR 702.40a; Stormscale Scion): the spell's cast trigger
// copies it once for each spell cast before it this turn, by any player, the
// copies are not cast (they fire no cast trigger and count as none), and each
// copy's controller chooses new targets where the spell declares them (CR
// 707.10c). Scion cast as the turn's third spell makes two 4/4 Dragon tokens
// beside it; as the first spell, none. A copy of the permanent spell resolves
// into a token (CR 707.10f); Chatterstorm's copies make Goblins, Brain
// Freeze's ask their targets.
var stormPrintedDecks = [2][]cardset.Entry{
	{
		{Name: "Stormscale Scion", Count: 4}, {Name: "Chatterstorm", Count: 3}, {Name: "Brain Freeze", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Elite Vanguard", Count: 2}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Lightning Bolt", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Elite Vanguard", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
	},
}

func TestPrintedStormLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stormprinted", stormPrintedDecks, 64, "KSTORMPRINTED_SEEDS")
	lockstepSet(t, "stormprinted", [2][]cardset.Entry{stormPrintedDecks[1], stormPrintedDecks[0]}, 64, "KSTORMPRINTED_SEEDS")
}

// TestPrintedStormMutation proves the lockstep reads the printed storm trigger.
func TestPrintedStormMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugIgnoreStorm = true
	defer func() { dirac.DebugIgnoreStorm = false }()
	if !lockstepDiverges(t, "stormprinted", stormPrintedDecks, 64) {
		t.Fatal("ignoring the printed storm trigger left the lockstep clean")
	}
}
