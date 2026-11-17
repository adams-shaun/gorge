package cardset_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMatchupDropsLandTypeSetterItsCardsDoNotReach: the pool's override list is
// copied into every matchup table, so a land-type setter record (Blood Moon's
// CR 305.7 override) that no card of the matchup reaches must not arrive as a
// setter: the kernel reads the flag in prepLandSet and prepLandGrant, and a
// stray record turned on its land-type grant model (and refused an attack tax
// beside it) in a table with no setter in it.
func TestMatchupDropsLandTypeSetterItsCardsDoNotReach(t *testing.T) {
	set := cardsettest.Set(t, "globallandtypesetter")
	setters := func(entries ...string) (n int) {
		var es []cardset.Entry
		for _, e := range entries {
			es = append(es, cardset.Entry{Name: e, Count: 1})
		}
		tab, _, _, err := set.Matchup(es, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range tab.Overrides {
			if o.SetLandSub {
				n++
			}
		}
		return n
	}
	if got := setters("Blood Moon", "Mountain", "Raging Goblin"); got == 0 {
		t.Fatal("control: the matchup holding Blood Moon carries no setter record")
	}
	if got := setters("Mountain", "Raging Goblin"); got != 0 {
		t.Fatalf("a matchup without a setter card carries %d setter records", got)
	}
}
