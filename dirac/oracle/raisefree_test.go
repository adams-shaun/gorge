package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The cost increase beside a free cast lockstep: a free cast pays the
// RaiseCost statics on what it still owes (CR 601.2f, 118.9d), where gorge
// composes them on the empty cost. Deck A's Thalia, the Survivor taxes the
// noncreature spells deck B casts for free through Omnipresence; Dragon's
// Prey costs {2} more against a Dragon, and Shiko casts a copy of it free.
var kraisefreeDecks = [2][]cardset.Entry{
	{
		{Name: "Thalia, the Survivor", Count: 4}, {Name: "Dragon's Prey", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Opt", Count: 3}, {Name: "Lightning Strike", Count: 3},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Helm of Awakening", Count: 3}, {Name: "Shivan Dragon", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Omnipresence", Count: 4}, {Name: "Shiko, Paragon of the Way", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Opt", Count: 3}, {Name: "Lightning Strike", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Dragon's Prey", Count: 1},
		{Name: "Forest", Count: 7}, {Name: "Mountain", Count: 7},
	},
}

func TestRaiseFreeLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "raisefree", kraisefreeDecks, 60, "KRAISEFREE_SEEDS")
}
