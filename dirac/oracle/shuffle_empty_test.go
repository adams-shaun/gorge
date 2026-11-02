package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Shuffle your graveyard into your library" makes you shuffle whatever the graveyard
// holds, nothing included: Feldon's Cane moves the deal of the cards under it either way.
func TestYourGraveyardSweepShufflesWhenEmptyLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	entries := []cardset.Entry{{Name: "Feldon's Cane", Count: 16}, {Name: "Plains", Count: 24}}
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
	empty := 0
	for seed := uint64(1); seed <= 16; seed++ {
		m, err := NewMatchup(reg, [2]string{"a", "b"}, [2][]*cards.Card{deck, deck}, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: true}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		at, grave := 0, [2]int{}
		check := func(g *GorgeSide, _ Side) *Divergence {
			for _, e := range g.E.L.Events[at:] {
				if e.Kind == events.Shuffle && grave[e.Player] == 0 {
					empty++
				}
			}
			at = len(g.E.L.Events)
			grave[0], grave[1] = len(g.E.G.Zone(state.ZGraveyard, 0)), len(g.E.G.Zone(state.ZGraveyard, 1))
			return nil
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 600, MaxTurns: 30, Check: check})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
		}
	}
	if empty == 0 {
		t.Fatal("no game shuffled an empty graveyard in")
	}
	t.Logf("%d shuffles of an empty graveyard", empty)
}
