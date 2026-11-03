package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The excess of a spell's damage (CR 120.10) is what was dealt beyond lethal, after
// replacement: Tomik's "plus 1" to a permanent an opponent controls makes Violent
// Echoes' six into seven, and the Jace it empowers carries the larger excess.
func TestExcessDamageCountsTheDamageDealtLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fra")
	entries := []cardset.Entry{{Name: "Tomik, Izzet Sparkmage", Count: 12}, {Name: "Violent Echoes", Count: 10},
		{Name: "Mountain", Count: 18}}
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
		m, err := NewMatchup(reg, [2]string{"exc-a", "exc-b"}, [2][]*cards.Card{deck, deck}, 0)
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
