package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A land whose only mana ability is "{T}, Pay 1 life: Add one mana of any
// color" (Mana Confluence): gorge's planner prices it as a last-resort source
// at its life (Consequence{Life: 1}, no normal alternative), so a plan uses it
// only when the basics cannot pay (Serra Angel's {W}, Air Elemental's {U},
// Counterspell's {U}{U}), never when the payment would be lethal; burn drives
// life into that guard. Mana Leak's unless window counts the source through
// gorge's paid-cost layer (each counted only while the life committed by the
// earlier ones and its own leave the payer alive; dirac painAvailMaskAt):
// seeds 964 and 1520 pin it. A paid-life source cannot be tapped below its
// life (CR 119.4): seed 2769 pays Mana Leak at life 1 with one Confluence and
// never a second. The expected answer at every decision is gorge's.
var kmanaConfluenceDecks = [2][]cardset.Entry{
	{
		{Name: "Mana Confluence", Count: 6}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		{Name: "Serra Angel", Count: 4}, {Name: "Air Elemental", Count: 3}, {Name: "Counterspell", Count: 2},
		{Name: "Mana Leak", Count: 2}, {Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 4},
	},
	{
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mana Leak", Count: 1},
	},
}

func TestManaConfluenceLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "manaconfluence", kmanaConfluenceDecks, 2800, "KMANACONFLUENCE_SEEDS")
}
