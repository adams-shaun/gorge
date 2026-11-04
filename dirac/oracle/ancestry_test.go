package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Path of Ancestry lockstep: it enters tapped, and its {T} ability adds
// one mana of a colour in its controller's commander's colour identity,
// which is undefined without a commander (CR 903.4f): the ability adds
// nothing, and its scry when that mana is spent never triggers. Both decks
// carry it beside Command Tower, also a source of nothing here, and enough
// basics to cast. The expected answer at every decision is gorge's.
var kancestryDecks = [2][]cardset.Entry{
	{
		{Name: "Path of Ancestry", Count: 8}, {Name: "Command Tower", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Path of Ancestry", Count: 8}, {Name: "Savannah Lions", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
	},
}

func TestAncestryLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ancestry", kancestryDecks, 400, "KANCESTRY_SEEDS")
}
