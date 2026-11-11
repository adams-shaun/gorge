//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatGiftLockstep plays four-seat BLB pods of gift cards dirac
// against gorge: the election offers one promise per opponent (gorge
// giftAsk), and the promised opponent is the one that draws (Wear Down,
// Peerless Recycling), gets the token (Valley Rally) or the permanent's
// gift trigger gives to (Scrapshooter, Starforged Sword). No divergence, no
// stall, and some game promises a gift to an opponent other than the first.
func TestFourSeatGiftLockstep(t *testing.T) {
	reg := synthReg(t)
	green := deckOf(6, "Wear Down", deckOf(6, "Peerless Recycling", deckOf(4, "Scrapshooter", deckOf(24, "Forest")...)...)...)
	red := deckOf(8, "Valley Rally", deckOf(8, "Starforged Sword", deckOf(24, "Mountain")...)...)
	decks := [4][]string{green, red, green, red}
	set := cardsettest.Synth(t, "std-blb")
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := chooseCode("gift_promise")
	promise := ActChoose + CanonKind(code)
	later := 0
	for seed := uint64(1); seed <= 8; seed++ {
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
			if a.Kind() == promise && a.Mode() > 0 {
				later++
			}
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	if later == 0 {
		t.Fatal("no game promised a gift to an opponent past the first")
	}
	t.Logf("%d promises past the first opponent", later)
}
