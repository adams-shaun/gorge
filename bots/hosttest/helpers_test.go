package hosttest_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/bots/azmcts"
	_ "github.com/adams-shaun/gorge/bots/castprofile"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/searchprobe"
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

// azWatchProbe installs azmcts.Watch for one leak row: a diagnostic of a
// searched kind with Stats.Searched == 1 is a real search. restore puts the
// previous hook back.
func azWatchProbe() (*host.SearchProbe, func()) {
	var mu sync.Mutex
	var diags []azmcts.Diag
	prev := azmcts.Watch
	azmcts.Watch = func(dg azmcts.Diag) {
		mu.Lock()
		defer mu.Unlock()
		diags = append(diags, dg)
	}
	return &host.SearchProbe{
		Mark: func() int {
			mu.Lock()
			defer mu.Unlock()
			return len(diags)
		},
		Searched: func(from, to int) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, dg := range diags[from:to] {
				if dg.Kind != "" && dg.Stats.Searched == 1 {
					return true
				}
			}
			return false
		},
	}, func() { azmcts.Watch = prev }
}

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

// beliefDecks is the closed-list test belief corpus: the five Foundations
// repo decks.
var beliefDecks = []string{
	"foundations-calling-all-angels", "foundations-keen-engineering", "foundations-reign-of-dragons",
	"foundations-tramplesaurus-rex", "foundations-wretched-ranks",
}

var (
	azMu     sync.Mutex
	azBelief *searchprobe.Belief
	azOn     = map[string]bool{}
)

// enableAZ registers az-c3 (open lists, as gorged serves it without
// -az-c3-belief), or az-c4/az-c5 (closed under the test belief), over a
// synthetic network, once per test binary, and returns the belief (built
// from the corpus for the closed policies; nil for az-c3).
func enableAZ(t *testing.T, policy string) *searchprobe.Belief {
	t.Helper()
	azMu.Lock()
	defer azMu.Unlock()
	closed := policy != azc3.Policy
	if closed && azBelief == nil {
		reg := testutil.CorpusRegistry(t)
		var decks [][]*cards.Card
		for _, n := range beliefDecks {
			decks = append(decks, testutil.RepoDeck(t, reg, n))
		}
		b, err := searchprobe.NewBelief(decks)
		if err != nil {
			t.Fatal(err)
		}
		azBelief = b
	}
	if !azOn[policy] {
		dir, err := os.MkdirTemp("", policy+"-host-")
		if err != nil {
			t.Fatal(err)
		}
		// The network is read whole at Enable; the file is not needed after.
		defer os.RemoveAll(dir)
		path, err := gonettest.Write(dir, "net.dzgw")
		if err != nil {
			t.Fatal(err)
		}
		switch policy {
		case azc3.Policy:
			err = azc3.Enable(path)
		case azc4.Policy:
			err = azc4.Enable(path, azBelief)
		case azc5.Policy:
			err = azc5.Enable(path, azBelief)
		default:
			t.Fatalf("enableAZ: %q is not an az policy", policy)
		}
		if err != nil {
			t.Fatal(err)
		}
		azOn[policy] = true
	}
	if _, ok := bots.Lookup(policy); !ok {
		t.Fatalf("%s is not registered", policy)
	}
	if !closed {
		return nil
	}
	return azBelief
}
