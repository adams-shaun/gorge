package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// The creature type count lockstep: Count$Valid ...$CreatureType read by a dig, by
// damage and by a cost, over printed types, a Borg the Federation
// cards skip, a Changeling beside the reader and Mirror Entity's grant of every type.
var ktypecountDecks = [2][]cardset.Entry{
	{
		{Name: "Hoshi Sato, Exolinguist", Count: 6}, {Name: "Saavik, Stoic Student", Count: 5}, {Name: "Plains", Count: 10},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Assault Drone", Count: 2}, {Name: "Universal Automaton", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Valiant Changeling", Count: 5}, {Name: "Saavik, Stoic Student", Count: 3}, {Name: "Changeling Outcast", Count: 5},
		{Name: "Mirror Entity", Count: 4}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
	},
}

func TestTypeCountLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("typecount")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktypecountDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ktypecountDecks[0], ktypecountDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktypecount-a", "ktypecount-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		rnd := RandomPolicy(seed)
		policy := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				for _, id := range c.G.E.G.Stack {
					if o := c.G.E.G.Obj(id); o != nil && o.Zone == state.ZStack && o.Face() != nil {
						seen[o.Face().Name]++
					}
				}
			}
			return rnd.Pick(c)
		})
		res := LockstepLed(m, seed, g, k, policy, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" && res.Stall != "turns" {
			t.Logf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
	}
	for _, name := range []string{"Hoshi Sato, Exolinguist", "Saavik, Stoic Student", "Valiant Changeling"} {
		if seen[name] == 0 {
			t.Errorf("%s never reached the stack: %v", name, seen)
		}
	}
}

// TestCreatureTypeWordsAreGorges: a creature type dirac's list lacks survives the
// loss of all creature types and misses this count.
func TestCreatureTypeWordsAreGorges(t *testing.T) {
	for _, w := range effects.CreatureTypeWordList() {
		if !dirac.IsCreatureTypeWord(w) {
			t.Errorf("%s is a creature type in gorge, not in dirac", w)
		}
	}
}
