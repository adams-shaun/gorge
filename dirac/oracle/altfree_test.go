package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The free alternative cost lockstep: Land Grant's "if you
// have no land cards in hand, you may reveal your hand rather than pay this
// spell's mana cost" (gorge alternativeCosts' face walk: an AltCostIndex 1
// cast while Count$ValidHand Land.YouOwn is 0), in a land-light green deck
// that empties its hand. The expected answer at every decision is gorge's.
var kaltfreeDecks = [2][]cardset.Entry{
	{
		{Name: "Land Grant", Count: 10}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Giant Growth", Count: 4}, {Name: "Forest", Count: 14},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 8},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestAltfreeLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "altfree", kaltfreeDecks, 1000, "KALTFREE_SEEDS")
}

// TestAltfreeLockstepMutation proves the pool exercises the free cast:
// with the alternative cost dropped, or its no-lands condition dropped,
// some seed diverges.
func TestAltfreeLockstepMutation(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Static)
	}{
		{"no alternative", func(st *kop.Static) { st.Kind = kop.StatNone }},
		{"no condition", func(st *kop.Static) { st.Cond2 = 0 }},
	} {
		n := lockstepRun(t, "altfree", kaltfreeDecks, 200, false, func(tbl *kop.Table) {
			for i := range tbl.Cards {
				for j := range tbl.Cards[i].Abilities {
					if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
						m.fn(&a.Static)
					}
				}
			}
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never exercises Land Grant's free cast", m.name)
		}
	}
}
