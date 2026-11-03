package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The dilemma lockstep: Seven of Nine's trigger and The Caretaker's count, over
// charms on spells and on a triggered ability.
var kdilemmaDecks = [2][]cardset.Entry{
	{
		{Name: "Seven of Nine", Count: 6}, {Name: "Abrade", Count: 6}, {Name: "Charming Scoundrel", Count: 6},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "The Caretaker", Count: 6}, {Name: "Seven of Nine", Count: 2}, {Name: "Abrade", Count: 6}, {Name: "Charming Scoundrel", Count: 6},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4},
	},
}

func TestDilemmaLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dilemma")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdilemmaDecks {
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
	tbl, _, _, err := set.Matchup(kdilemmaDecks[0], kdilemmaDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	faced := int32(0)
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdilemma-a", "kdilemma-b"}, decks, 1)
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
			faced = max(faced, c.G.E.G.Players[0].DilemmasFacedThisTurn, c.G.E.G.Players[1].DilemmasFacedThisTurn)
			return rnd.Pick(c)
		})
		res := LockstepLed(m, seed, g, k, policy, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Stall != "" && res.Stall != "turns" {
			t.Fatalf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
	}
	if faced < 2 {
		t.Errorf("at most %d dilemmas in a turn, want a turn with two", faced)
	}
}
