package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A mana ability that adds several colours at once (Timeless Lotus's
// {W}{U}{B}{R}{G}) is one fixed-production planner alternative in gorge whose
// mana holds every colour: the kernel's availability, plan, ranks and taps must
// read all of it, a land's too (a Karoo's {U}{R}, Coral Atoll's {C}{U}). The basic
// lands make only {G} (and {U}), so most other colours the spells need come from
// a Lotus or a Karoo.
var keachmanaDecks = [2][]cardset.Entry{
	{
		{Name: "Timeless Lotus", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Gravedigger", Count: 3}, {Name: "Divination", Count: 2}, {Name: "Shock", Count: 3},
		{Name: "Duress", Count: 2}, {Name: "Serra Angel", Count: 1}, {Name: "Coral Atoll", Count: 2}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Timeless Lotus", Count: 4}, {Name: "Elvish Mystic", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Giant Growth", Count: 4}, {Name: "Shivan Dragon", Count: 2}, {Name: "Air Elemental", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Izzet Boilerworks", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 2},
	},
}

func TestEachColourManaLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "eachmana", keachmanaDecks, 300, "KEACHMANA_SEEDS")
}

// TestEachColourManaLockstepMutation: read as one mana of any colour (the
// all-at-once flag dropped), the Lotus makes some seed diverge, so the pool
// does exercise the simultaneous production.
func TestEachColourManaLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "eachmana", keachmanaDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				abs[j].ManaEach = false
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the all-at-once production read as one colour")
	}
}
