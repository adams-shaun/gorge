package azc3_test

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAZSearchRatchet is the az search's failure ratchet: a fixed set of
// closed-list az-c4-config games (no network, a few simulations per
// searched decision), driven exactly as draftlab and cardval drive them
// (EnvSeats + HonestRoot), must finish with no recovered simulation panic,
// no feed-stopped decision, no world-less simulation and no decision whose
// every simulation failed. Each counter was nonzero before its root cause
// was fixed: nested-world tape re-runs (Panics), library-search redeal
// refusals (FeedStopped), the lazy payment build in the observation
// (NoWorld, AllFailed). A new nonzero count is a new bug, not noise.
func TestAZSearchRatchet(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{
		"foundations-tramplesaurus-rex", "foundations-wretched-ranks",
		"foundations-calling-all-angels", "foundations-reign-of-dragons",
	}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	belief, err := searchprobe.NewBelief(decks)
	if err != nil {
		t.Fatal(err)
	}
	cfg := azc3.ClosedConfig(belief)
	cfg.Search.Sims = 4

	var mu sync.Mutex
	var st azmcts.Stats
	searched := 0
	prev := azmcts.Watch
	azmcts.Watch = func(d azmcts.Diag) {
		mu.Lock()
		defer mu.Unlock()
		st.Add(d.Stats)
		if d.Searched {
			searched++
		}
	}
	t.Cleanup(func() { azmcts.Watch = prev })

	games := []struct {
		a, b int
		seed uint64
	}{{0, 1, 1}, {2, 3, 2}, {1, 2, 3}, {3, 0, 4}}
	for _, g := range games {
		seats := make([]seat.Seat, 2)
		for i := range seats {
			s, err := azredeal.New(bots.Options{Seed: g.seed*2 + uint64(i)}, cfg)
			if err != nil {
				t.Fatal(err)
			}
			seats[i] = s
		}
		rc := rules.Config{Seed: g.seed, Names: []string{names[g.a], names[g.b]},
			Decks: [][]*cards.Card{decks[g.a], decks[g.b]}, Tokens: reg.Tokens}
		if _, _, err := bench.PlayGame(rc, seats, 30, 0, bench.Hooks{EnvSeats: true, HonestRoot: true}); err != nil {
			t.Fatalf("game %v: %v", g, err)
		}
	}
	t.Logf("searched %d decisions: sims %d completed %d panics %d no-world %d all-failed %d feed-stopped %d (root-refused %d)",
		searched, st.Simulations, st.Completed, st.Panics, st.NoWorld, st.AllFailed, st.FeedStopped, st.RootRefused)
	if searched == 0 {
		t.Fatal("no decision was searched: the ratchet measures nothing")
	}
	if st.Panics != 0 || st.FeedStopped != 0 || st.RootRefused != 0 || st.NoWorld != 0 || st.AllFailed != 0 {
		t.Fatalf("az search failures: panics %d feed-stopped %d (true stops %d, root-refused %d) no-world %d all-failed %d",
			st.Panics, st.FeedStopped, st.FeedStopped-st.RootRefused, st.RootRefused, st.NoWorld, st.AllFailed)
	}
}
