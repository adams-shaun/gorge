package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The NoAbilities lockstep: Fang-Druid Summoner searches the library and
// graveyard for a creature card with no abilities, and Rise from the Wreck
// targets one in the graveyard. Grizzly Bears and Hill Giant have none;
// Llanowar Elves (a mana ability), Serra Angel (flying) and Savannah Lions
// (none either) are mixed in, against burn that fills the graveyard. The
// expected answer at every decision is gorge's.
var knoabDecks = [2][]cardset.Entry{
	{
		{Name: "Fang-Druid Summoner", Count: 6}, {Name: "Rise from the Wreck", Count: 6},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 6}, {Name: "Serra Angel", Count: 2},
		{Name: "Forest", Count: 12},
	},
	{
		{Name: "Shock", Count: 10}, {Name: "Hill Giant", Count: 6}, {Name: "Savannah Lions", Count: 6},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9},
	},
}

func TestNoAbilitiesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "noabilities", knoabDecks, 300, "KNOAB_SEEDS")
}

// TestNoAbilitiesLockstepMutation proves the pool reads the word: with every
// card taken to have abilities, some seed diverges.
func TestNoAbilitiesLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "noabilities", knoabDecks, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			tbl.Cards[i].NoAbilities = false
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with every card taken to have abilities: the pool never asks for a card with none")
	}
}
