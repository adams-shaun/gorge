package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Scratch control: the same decks without Aswan Jaguar, seed 1.
func TestZZChooseTypeCtlSeed1(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetypectl")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	deck := []cardset.Entry{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Savannah Lions", Count: 4},
		{Name: "Scryb Sprites", Count: 4}, {Name: "Giant Growth", Count: 8},
		{Name: "Forest", Count: 16},
	}
	for s := 0; s < 2; s++ {
		for _, e := range deck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatal(e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 30; seed++ {
		m, err := NewMatchup(reg, [2]string{"ctl-a", "ctl-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("ctl seed %d: diverged at %d: %s", seed, res.Div.At, res.Div.Field)
			break
		}
	}
}
