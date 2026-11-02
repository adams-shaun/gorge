package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A Treasure sacrificed to pay for a cost inside a resolution waits in the graveyard
// for the next state-based check (CR 704.5d; no check runs inside one resolution),
// and a decision that follows the payment shows it there: Descendant of Storms'
// "you may pay {1}{W}. If you do, it endures 1" pays, then asks counter or token.
func TestTreasureSacrificedPayingInAResolutionWaitsLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tdm")
	entries := []cardset.Entry{{Name: "Descendant of Storms", Count: 14}, {Name: "Meticulous Artisan", Count: 8},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9}}
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
	for seed := uint64(1); seed <= 60; seed++ {
		m, err := NewMatchup(reg, [2]string{"spw-a", "spw-b"}, [2][]*cards.Card{deck, deck}, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ExplorePolicy(seed), RunConfig{Config: Config{GorgeAutopaySurface: true}})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
		}
	}
}
