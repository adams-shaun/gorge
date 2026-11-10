//go:build dirac_mp

package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFourSeatDefendingLockstep plays four-seat OTJ pods of Attacks triggers
// that target a creature the defending player controls (Spring Splasher,
// Thunder Lasso) dirac against gorge. Past two seats the defending player is
// the seat the trigger's attacker attacks, not every seat but the active
// one: no divergence and no stall. Read as the seat after the active one
// (dirac.DebugDefendingActiveOpp), some seed diverges.
func TestFourSeatDefendingLockstep(t *testing.T) {
	reg := synthReg(t)
	blue := deckOf(12, "Spring Splasher", deckOf(4, "Thunder Lasso", deckOf(24, "Island")...)...)
	red := deckOf(12, "Spring Splasher", deckOf(4, "Thunder Lasso", deckOf(12, "Island", deckOf(12, "Mountain")...)...)...)
	decks := [4][]string{blue, red, blue, red}
	set := cardsettest.Synth(t, "std-otj")
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
	play := func(seed uint64) Result {
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
		defer CloseSide(k)
		return Lockstep(m, seed, k, RandomPolicy(seed), rc)
	}
	const seeds = 8
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		t.Logf("seed %d: %d decisions, over %v %s", seed, res.Decisions, res.Over, res.Stall)
	}
	dirac.DebugDefendingActiveOpp = true
	defer func() { dirac.DebugDefendingActiveOpp = false }()
	for seed := uint64(1); seed <= seeds; seed++ {
		if res := play(seed); res.Div != nil {
			t.Logf("mutation: seed %d diverges at %d (%s %s)", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("reading the defending player as the seat after the active one diverged on no seed")
}
