package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A creature dealt damage by a deathtouch source with wither takes the damage as
// -1/-1 counters and none marked, yet is destroyed all the same (CR 702.2b, 704.5g):
// Massacre Girl gives Rakish Scoundrel wither, and a Scoundrel that blocks or is
// blocked by a creature with toughness to spare kills it with three counters. The
// mark alone is lethal, as in gorge's destroyLethalDamage.
func TestDeathtouchWitherDamageDestroysLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-mkm")
	entries := []cardset.Entry{{Name: "Massacre Girl, Known Killer", Count: 6}, {Name: "Rakish Scoundrel", Count: 12},
		{Name: "Swamp", Count: 11}, {Name: "Forest", Count: 11}}
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
	for seed := uint64(1); seed <= 40; seed++ {
		m, err := NewMatchup(reg, [2]string{"dtw-a", "dtw-b"}, [2][]*cards.Card{deck, deck}, 0)
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
