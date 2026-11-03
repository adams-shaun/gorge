package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The hybrid cost paid from floating restricted mana (CR 106.6, 107.4e):
// Flamebraider adds two mana of any colours to spend on Elementals, and
// gorge resolves a hybrid pip over the pool that payment may spend. Catharsis
// is an Elemental evoked for {R/W}{R/W}, so a floating {R}{R} (or {R}{W})
// pays both pips and the lowering's witness taps no source for them: a
// witness that left the pips owing found no plan and the kernel panicked
// ("autopay found no plan"). The test counts the kernel's panics only; the
// pip ask beside an untapped Flamebraider is a separate gap, so a game may
// still diverge on its options.
var khybrestrDecks = [2][]cardset.Entry{
	{
		{Name: "Catharsis", Count: 6}, {Name: "Flamebraider", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 20},
	},
}

func TestHybrestrNoPlanPanic(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("hybrestr")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range khybrestrDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(khybrestrDecks[0], khybrestrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	panics := 0
	for seed := uint64(1); seed <= 300; seed++ {
		m, err := NewMatchup(reg, [2]string{"hybrestr-a", "hybrestr-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if d := res.Div; d != nil && d.What == "kernel" {
			panics++
			t.Errorf("seed %d: kernel panic at decision %d: %s", seed, d.At, d.Detail)
		}
	}
	t.Logf("300 games, %d kernel panics", panics)
}
