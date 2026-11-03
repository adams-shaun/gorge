package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A spell-cast trigger that reads the source's chosen colour (Diamond Mare: "whenever
// you cast a spell of the chosen color, you gain 1 life") fires on a spell with that
// colour. The filter's colour is the source's as-enters choice, so a card filter read
// the choice through no source and never matched.
func TestCastTriggerOfTheChosenColourLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	entries := []cardset.Entry{{Name: "Diamond Mare", Count: 10}, {Name: "Ajani's Pridemate", Count: 8},
		{Name: "Plains", Count: 11}, {Name: "Island", Count: 11}}
	var deck []*cards.Card
	for _, e := range entries {
		cd, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, cd)
		}
	}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 30; seed++ {
		m, err := NewMatchup(reg, [2]string{"ccc-a", "ccc-b"}, [2][]*cards.Card{deck, deck}, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: Config{GorgeAutopaySurface: true}})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
		}
	}
}
