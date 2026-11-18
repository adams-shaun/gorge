package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPainAnyColourLockstep: Tarnished Citadel ("{T}: Add one mana of any
// color. CARDNAME deals 3 damage to you") and Grand Coliseum (the same with
// 1 damage, enters tapped) in one deck. Both are painful any-colour units of
// different damage; the planner's pick among them must match gorge's.
func TestPainAnyColourLockstep(t *testing.T) {
	t.Parallel()
	pain := []cardset.Entry{
		{Name: "Tarnished Citadel", Count: 8}, {Name: "Grand Coliseum", Count: 8},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 4},
	}
	opp := []cardset.Entry{{Name: "Island", Count: 40}}
	decks := [2][]cardset.Entry{pain, opp}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				cs[s] = append(cs[s], c)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 24; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"painany-a", "painany-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
	}
}
