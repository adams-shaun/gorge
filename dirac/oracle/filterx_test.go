package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The filter X lockstep: X is announced before the mana window opens and is
// bounded by what the window can make (CR 601.2b, 601.2g), and a filter's fee is
// paid out of mana made earlier in it (CR 605.3b). Starfleet Academy's {5},{T}
// makes one colour as many times as its controller has creature types, so it
// raises the largest X past one mana a source; Hidden Grotto's {1},{T} trades its
// {C} for the red no other source of the second deck makes.
var kfilterxDecks = [2][]cardset.Entry{
	{
		{Name: "Starfleet Academy", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "House Cartographer", Count: 4}, {Name: "Saurian Explorer", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Engineered Explosives", Count: 3},
		{Name: "Mind Spring", Count: 3}, {Name: "Island", Count: 9}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Hidden Grotto", Count: 8}, {Name: "Blaze", Count: 4},
		{Name: "Crackle with Power", Count: 3}, {Name: "Stroke of Genius", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	},
}

func TestFilterXLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("filterx")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kfilterxDecks {
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
	tbl, _, _, err := set.Matchup(kfilterxDecks[0], kfilterxDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 96; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kfilterx-a", "kfilterx-b"}, decks, 1)
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
