package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Defined$ values kgen lower_definedcensus1125.go lowers, played in
// lockstep: Unquenchable Fury's granted "whenever this creature attacks"
// damages TriggeredDefender (gorge's c.DefendingPlayer, the kernel's
// RefDefendingPlayer) by the size of that player's hand, and Will of the
// Sultai's PutCounter mode gives TargetedCard (the chain's one creature
// target) trample.
var definedCensus1125Decks = [2][]cardset.Entry{
	{
		{Name: "Unquenchable Fury", Count: 8}, {Name: "Will of the Sultai", Count: 4},
		{Name: "Raging Goblin", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Will of the Sultai", Count: 6}, {Name: "Unquenchable Fury", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 8},
	},
}

func TestDefinedCensus1125Lockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	lockstepRun(t, "definedcensus1125", definedCensus1125Decks, 60, true, nil)
}
