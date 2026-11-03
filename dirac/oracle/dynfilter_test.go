package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The dynamic filter lockstep: Starfleet Academy's {5},{T} and Three Tree City's
// {2},{T} make an amount of one colour, so gorge's planner cannot price them and a
// play it proves insufficient takes its potential search, which can fund a colour
// no other source makes.
var kdynfilterDecks = [2][]cardset.Entry{
	{
		{Name: "Starfleet Academy", Count: 4}, {Name: "D'Vana Tendi, Optimistic Ensign", Count: 6},
		{Name: "Saurian Explorer", Count: 4}, {Name: "Xenobotanist", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Island", Count: 14}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Three Tree City", Count: 4}, {Name: "D'Vana Tendi, Optimistic Ensign", Count: 6},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 10},
	},
}

func TestDynamicFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dynfilter")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdynfilterDecks {
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
	tbl, _, _, err := set.Matchup(kdynfilterDecks[0], kdynfilterDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdynfilter-a", "kdynfilter-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
}
