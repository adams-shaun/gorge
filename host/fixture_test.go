package host

import (
	"os"
	"sync"
	"testing"
)

// TestMain closes the shared finished match below once, after every test
// that borrowed it has finished.
//
// Not here on purpose: debug.SetGCPercent. Tried at 400 (the suite spends
// ~50% of CPU in GC): a single match peaks near 1GB live, so with every
// test parallel the run thrashed instead — 99s, 82s of it system time.
// Parallelism is bounded by matchSlots instead.
func TestMain(m *testing.M) {
	code := m.Run()
	sharedFixture.mu.Lock()
	if sharedFixture.r != nil {
		sharedFixture.r.Close()
	}
	sharedFixture.mu.Unlock()
	os.Exit(code)
}

// matchSlots bounds how many registries — i.e. matches, each ~1GB peak and
// one core — exist at once across the parallel tests. testOptions takes a
// slot for the test's lifetime. 8 keeps a 32-core box busy without paging;
// the un-gated version (33 matches at once) was slower than sequential.
var matchSlots = make(chan struct{}, matchSlotCount())

// matchSlotCount is 8, or 4 under -race (raceEnabled, fixture_race_test.go):
// the race detector multiplies each match's memory several times over, and
// the only -race run is the pre-push gate on main (FL-56), where not paging
// matters more than wall time.
func matchSlotCount() int {
	if raceEnabled {
		return 4
	}
	return 8
}

func takeMatchSlot(t *testing.T) {
	t.Helper()
	matchSlots <- struct{}{}
	t.Cleanup(func() { <-matchSlots })
}

var sharedFixture struct {
	once sync.Once
	mu   sync.Mutex
	r    *Registry
	m    *match
	err  error
}

// finishedTable returns a Registry whose table "t1" has played exactly one
// match — fourSeatTable("t1", false), seed 99 — to completion, and that
// finished match. The match is played ONCE per test binary and shared:
// seed 99 makes it a pure function of the configuration
// (TestTheSameConfigurationPlaysTheSameMatch), and every caller only reads
// (Registry.ViewAt/Events, the finished engine's log, bounds, snapshots).
//
// Callers must not Start, AddTable, Close, or otherwise mutate the
// registry or the match; a test that needs a table of its own (a live
// one, a Public one, a second table) builds it with New(testOptions(t)) as
// before. The registry is closed by TestMain, not by any one test.
func finishedTable(t *testing.T) (*Registry, *match) {
	t.Helper()
	sharedFixture.once.Do(func() {
		r, err := New(testOptions(t))
		if err != nil {
			sharedFixture.err = err
			return
		}
		if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
			r.Close()
			sharedFixture.err = err
			return
		}
		if err := r.Start("t1"); err != nil {
			r.Close()
			sharedFixture.err = err
			return
		}
		r.Wait("t1")
		r.mu.RLock()
		tb := r.tables["t1"]
		r.mu.RUnlock()
		sharedFixture.mu.Lock()
		sharedFixture.r, sharedFixture.m = r, tb.history[0]
		sharedFixture.mu.Unlock()
	})
	if sharedFixture.err != nil {
		t.Fatalf("shared finished table: %v", sharedFixture.err)
	}
	return sharedFixture.r, sharedFixture.m
}
