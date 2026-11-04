package hosttest_test

import (
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	_ "github.com/adams-shaun/gorge/bots/castprofile"
	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/bots/spellbench/sbsearch"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// testOptions is server/host's testOptions: testutil.SampleDecks'
// four decks served under "a".."d" (corpus-free, deterministic) and a sleep
// that never waits.
func testOptions(t *testing.T) host.Options {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	return host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			cs, ok := byName[name]
			if !ok {
				return host.Deck{}, host.ErrNotFound
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	}
}

// sampleBotDeps is a cards registry over the same sample deck pool, the card
// dependency a policy whose factory reads card facts (the sb-* adapters)
// resolves from.
func sampleBotDeps(t *testing.T) bots.Deps {
	t.Helper()
	_, decks := testutil.SampleDecks(t, 4)
	reg := cards.NewRegistry()
	for _, cs := range decks {
		for _, c := range cs {
			reg.Add(c)
		}
	}
	return bots.Deps{Cards: reg}
}

// newSeat builds the policy's seat or fails the test.
func newSeat(t *testing.T, policy string, o bots.Options) seat.Seat {
	t.Helper()
	s, err := bots.New(policy, o)
	if err != nil {
		t.Fatalf("the %s policy is not linked into this binary: %v", policy, err)
	}
	return s
}

// builtinsPlanner is EnvLeakContract.PlannerOf for an adapter over a
// SpellBench builtins.Seat.
func builtinsPlanner(t *testing.T) func(seat.Seat) any {
	return func(s seat.Seat) any {
		ad, ok := s.(interface{ Seat() *builtins.Seat })
		if !ok {
			t.Fatalf("%T does not expose its wrapped builtins seat", s)
		}
		return ad.Seat().Planner()
	}
}

// watchMu serialises the rows that own a package-level search hook
// (azmcts.Watch, sbsearch.Watch).
var watchMu sync.Mutex

// sbWatchProbe installs sbsearch.Watch for one leak row: sbsearch emits a
// Diag only for a decision it searched.
func sbWatchProbe() (*host.SearchProbe, func()) {
	var mu sync.Mutex
	var n int
	prev := sbsearch.Watch
	sbsearch.Watch = func(sbsearch.Diag) {
		mu.Lock()
		defer mu.Unlock()
		n++
	}
	return &host.SearchProbe{
		Mark: func() int {
			mu.Lock()
			defer mu.Unlock()
			return n
		},
		Searched: func(from, to int) bool { return to > from },
	}, func() { sbsearch.Watch = prev }
}
