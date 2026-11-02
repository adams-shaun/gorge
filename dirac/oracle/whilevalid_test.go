package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The conditional exile permission's lockstep: Possibility Technician exiles the
// top card of the library, and its controller may play that card for as long as it
// stays exiled while they control a Kavu (CR 611.2b). The opponent's removal takes
// the Kavu away, withdrawing the offer until another one arrives.
var kwhilevalidDecks = [2][]cardset.Entry{
	{
		{Name: "Possibility Technician", Count: 16}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Terror", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

func TestWhilevalidLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("whilevalid")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwhilevalidDecks {
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
	tbl, _, _, err := set.Matchup(kwhilevalidDecks[0], kwhilevalidDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 192; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kwhilevalid-a", "kwhilevalid-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, poisonhighPolicy(seed, map[string]int{}, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
}
