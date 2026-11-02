package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The assimilate lockstep: Borg Queen takes a creature card from an opponent's
// graveyard, and Reckless Fireweaver sees an artifact enter. Two Queens a deck: a
// pair taking each other back runs into gorge's trigger cap (maxTriggerFires), which
// the kernel does not count.
var kassimilateDecks = [2][]cardset.Entry{
	{
		{Name: "Borg Queen, Perfection Manifest", Count: 2}, {Name: "Reckless Fireweaver", Count: 8}, {Name: "Lightning Bolt", Count: 10},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Borg Queen, Perfection Manifest", Count: 2}, {Name: "Grizzly Bears", Count: 9}, {Name: "Goblin Piker", Count: 9},
		{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 7}, {Name: "Mountain", Count: 7},
	},
}

func TestAssimilateLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("assimilate")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kassimilateDecks {
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
	tbl, _, _, err := set.Matchup(kassimilateDecks[0], kassimilateDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kassimilate-a", "kassimilate-b"}, decks, 1)
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
