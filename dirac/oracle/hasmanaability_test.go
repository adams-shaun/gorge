package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The hasManaAbility lockstep: Power Sink's unless-payment decline taps the
// targeted player's lands that have mana abilities, Moonsilver Key searches
// an artifact with one (or a basic land), and Mana Short taps every land and
// drains the pool. Basics, Sol Ring and Llanowar Elves are the matching
// sources; Grizzly Bears, Air Elemental and the spells are the non-matches.
var khmDecks = [2][]cardset.Entry{
	{
		{Name: "Power Sink", Count: 6}, {Name: "Moonsilver Key", Count: 4},
		{Name: "Mana Short", Count: 2}, {Name: "Sol Ring", Count: 4},
		{Name: "Counterspell", Count: 4}, {Name: "Cancel", Count: 2},
		{Name: "Air Elemental", Count: 2}, {Name: "Island", Count: 10},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Shock", Count: 6}, {Name: "Forest", Count: 12},
		{Name: "Mountain", Count: 10},
	},
}

func TestHasManaAbilityLockstep(t *testing.T) {
	t.Parallel()
	for s, d := range khmDecks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d has %d cards, want 40", s, n)
		}
	}
	lockstepSet(t, "hasmanaability", khmDecks, 120, "KHM_SEEDS")
}

// TestHasManaAbilityLockstepMutation proves the pool reads the property: with
// every card taken to have no mana ability, a seed diverges.
func TestHasManaAbilityLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "hasmanaability", khmDecks, 120, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			tbl.Cards[i].HasManaAbility = false
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with every card taken to have no mana ability: the pool never asks for one")
	}
}
