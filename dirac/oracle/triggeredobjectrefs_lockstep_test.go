package oracle

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Defined$ Triggered* object references play identically in gorge and
// dirac (dirac/kgen/lower_trigrefs.go): the damage carriers read
// TriggeredSourceController (Phyrexian Obliterator, Belltower Sphinx) and
// TriggeredTarget (Charisma, Sword of Kaldra), the BecomesTarget watcher
// reads the targeted permanent (Cloud Cover), and Ferocity's
// Blocks trigger reads the blocker as its host. gorge leads every seed; the
// policy casts a carrier whenever one is offered, then falls back to
// castFirstPolicy, and the test fails when a carrier is never cast.
var triggeredObjectRefCards = map[string]bool{
	"Phyrexian Obliterator": true, "Belltower Sphinx": true, "Charisma": true,
	"Cloud Cover": true, "Sword of Kaldra": true,
	"Ferocity": true,
}

var triggeredObjectRefDecks = [2][]cardset.Entry{
	{
		{Name: "Phyrexian Obliterator", Count: 4}, {Name: "Belltower Sphinx", Count: 4},
		{Name: "Charisma", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Swamp", Count: 9}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Cloud Cover", Count: 4}, {Name: "Sword of Kaldra", Count: 3},
		{Name: "Ferocity", Count: 3}, {Name: "Belltower Sphinx", Count: 2},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7}, {Name: "Forest", Count: 6},
	},
}

func triggeredObjectRefPolicy(seed uint64, played map[string]int) Policy {
	base := castFirstPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(o.Obj)
				if ob == nil || ob.Face() == nil || !triggeredObjectRefCards[ob.Face().Name] {
					continue
				}
				played[ob.Face().Name]++
				return a
			}
		}
		return base.Pick(c)
	})
}

func TestTriggeredObjectRefLockstep(t *testing.T) {
	t.Parallel()
	played := map[string]int{}
	if n := lockstepRunPolicy(t, "triggeredobjectrefs", triggeredObjectRefDecks, 200, true, nil, nil,
		func(seed uint64) Policy { return triggeredObjectRefPolicy(seed, played) }); n != 0 {
		t.Fatalf("%d divergent seeds", n)
	}
	names := make([]string, 0, len(triggeredObjectRefCards))
	for name := range triggeredObjectRefCards {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if played[name] == 0 {
			t.Errorf("%s was never cast: the decks do not reach it", name)
		}
	}
}
