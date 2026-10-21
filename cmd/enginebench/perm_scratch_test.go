package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRandomIntentScratchKeepsTheStream pins the random row's per-seed
// outcome after W3 (action-walk.md §6): replacing math/rand/v2's Perm
// allocation with a reused in-place buffer must not move the RNG stream, so
// every seed still plays the identical game. The counts below were taken on
// base 2cb30d4bf (before W3) and must not move.
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
	// deterministic, so the totals are too. Re-pinned for the paid-cost mana
	// census default flip (2026-10-13; was 1600/38249, 1621/38800, 1648/40117):
	// the census offers more options, so the random games move. Forcing
	// engine/rules/genesis.go's paidManaCensus to false restores the old totals.
	// Re-pinned for the combat priority fixes (rules e7aaf1530 and 93b6057e4,
	// CR 510.4/117.3a; was 1616/38711, 1651/39748, 1620/39304): more priority
	// windows in combat draw more random answers, so the games move.
	// Re-pinned for the CR 510.3 post-damage priority round (rules
	// 24bbba791; was 1644/41005, 1669/41915, 1700/43169), the same cause.
	// Re-pinned after cr-g69 (CR 601.2h: cost objects are chosen at payment,
	// after the targets and the mana window): the random games answer the
	// reordered decisions differently, so the games move.
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
