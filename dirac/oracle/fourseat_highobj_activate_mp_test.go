//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatHighObjectActivationLockstep plays four-seat pods of 100-card
// decks dirac against gorge, each deck 40 Mind Stones and 60 Islands, so
// seats 2 and 3 own objects 200..399. Mind Stone's draw ability is its
// kernel ability 0 and its second gorge ability line (the mana ability comes
// first), so the canonical activation reads the object's card. The
// translator read the source through a byte: an activation of object 300 named
// object 44's card, and in 4 x 100 MBC pods 89 of 200 games diverged on
// "options" (gorge activate(#318,-,1), kernel activate(#318,-,255)). No
// divergence, no kernel stall, and some activation's source is past 255.
func TestFourSeatHighObjectActivationLockstep(t *testing.T) {
	reg := synthReg(t)
	d := deckOf(40, "Mind Stone", deckOf(60, "Island")...)
	decks := [4][]string{d, d, d, d}
	set := cardsettest.Set(t, "cmd5")
	var lists [][]cardset.Entry
	for _, dk := range decks {
		es := make([]cardset.Entry, len(dk))
		for i, n := range dk {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	high := 0
	for seed := uint64(1); seed <= 4; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 3000, 30
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), rc)
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		for _, a := range res.Actions {
			if a.Kind() == ActActivate && a.Src() >= 256 && a.Src() < RefNone {
				high++
			}
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if high == 0 {
		t.Fatal("no activation of an object past 255")
	}
	t.Logf("%d activations of objects past 255", high)
}
