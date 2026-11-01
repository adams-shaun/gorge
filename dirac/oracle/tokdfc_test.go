package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The token / back-face lockstep: Fable of the Mirror-Breaker's
// chapters and its back face Reflection of Kiki-Jiki (a hasty token copy
// sacrificed at the next end step: CopyPermanent AtEOT$ Sacrifice, gorge
// scheduleAtEOT's __kwEncoreSacrifice), and Reckoner Bankbuster's draw
// ability whose last charge counter creates a Treasure and a crew-buffed
// Pilot (a comma-listed TokenScript$ under ConditionPresent$
// Card.counters_EQ0_CHARGE: PredCountersLT), and Thatcher Revolt / Hungry
// for More's Token AtEOT$ Sacrifice (the same untracked end-step record),
// beside creatures to copy, burn and removal. The expected answer at every decision is gorge's.
var ktokdfcDecks = [2][]cardset.Entry{
	{
		{Name: "Fable of the Mirror-Breaker", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Thatcher Revolt", Count: 2},
		{Name: "Hungry for More", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 2}, {Name: "Doom Blade", Count: 2},
	},
	{
		{Name: "Reckoner Bankbuster", Count: 4}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 8},
		{Name: "Savannah Lions", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Hill Giant", Count: 4},
	},
}

func TestTokdfcLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tokdfc", ktokdfcDecks)
	p3Lockstep(t, "tokdfc", reg, set, decks, tbl, 600)
}
