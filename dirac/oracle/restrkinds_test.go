package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A table with three spend restrictions (Villainous Hideout's Villain-only
// mana, Avengers Tower's Hero-only mana, Castle Doom's artifact-only mana)
// floats each kind's restricted mana as restricted: the kernel names every
// kind a Standard table holds, so no game stalls at capacity and every
// payment matches gorge's (CR 106.6).
func TestThreeSpendRestrictionsLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-msh")
	entries := []cardset.Entry{{Name: "Villainous Hideout", Count: 7}, {Name: "Avengers Tower", Count: 7},
		{Name: "Castle Doom", Count: 7}, {Name: "Surveillance Room", Count: 3}, {Name: "A.I.M. Synthoids", Count: 8}, {Name: "Killmonger, Scourge of Wakanda", Count: 6},
		{Name: "Stature, Size Shifter", Count: 6}, {Name: "Ka-Zar of the Savage Land", Count: 6},
		{Name: "Claim the Kingdom", Count: 4}, {Name: "Wakandan Royal Guard", Count: 4}}
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
	for _, autopay := range []bool{false} { // the full surface: a player taps the restricted lands first
		// the seeds whose full-surface games float the third kind of mana
		for _, seed := range []uint64{6, 9, 10, 11, 15, 20, 23, 24} {
			m, err := NewMatchup(reg, [2]string{"trk-a", "trk-b"}, [2][]*cards.Card{deck, deck}, 0)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: autopay}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg})
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("autopay=%v seed %d: divergence at %d: %s %s %s %v", autopay, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("autopay=%v seed %d: %s", autopay, seed, res.Stall)
			}
		}
	}
}
