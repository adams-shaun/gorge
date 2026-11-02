package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRandomIntentScratchKeepsTheStream pins the random row's per-seed
// outcome: replacing math/rand/v2's Perm allocation with a reused in-place
// buffer must not move the RNG stream, so every seed still plays the identical
// game.
func TestRandomIntentScratchKeepsTheStream(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	a, err := testutil.LoadRepoDeck(reg, "mono-black-aggro")
	if err != nil {
		t.Fatal(err)
	}
	b, err := testutil.LoadRepoDeck(reg, "mono-red-prowess")
	if err != nil {
		t.Fatal(err)
	}
	// fixed seeds and a fixed game budget: the draws inside each game are
	// deterministic, so the totals are too. A rules change that alters what a
	// random game is offered (a paid-cost census, a priority window, a reordered
	// decision) moves them; a scratch-buffer change must not.
	want := []struct {
		games, turns, decisions, rejected int
	}{
		{40, 1647, 41272, 0},
		{40, 1645, 41090, 0},
		{40, 1621, 40818, 0},
	}
	for i, w := range want {
		cfg := rules.Config{
			Seed:   77123 + uint64(i)*1000,
			Names:  []string{"a", "b"},
			Decks:  [][]*cards.Card{a, b},
			Tokens: reg.Tokens,
		}
		var st randomStats
		for g := 0; g < w.games; g++ {
			cfg.Seed = 77123 + uint64(i)*1000 + uint64(g)
			playRandom(cfg, &st)
		}
		if st.Games != w.games || st.Turns != w.turns || st.Decisions != w.decisions || st.Rejected != w.rejected {
			t.Errorf("seed set %d: games=%d turns=%d decisions=%d rejected=%d, want %d/%d/%d/%d",
				i, st.Games, st.Turns, st.Decisions, st.Rejected, w.games, w.turns, w.decisions, w.rejected)
		}
	}
}
