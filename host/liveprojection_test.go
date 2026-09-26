package host

// Regression for the live-projection race: a focus client joining, or a
// match starting/rewinding while clients watch, could crash its table with
// an intermittent module-gate panic in rules.(*Engine).Derived /
// FilterDerivedPT. view.ProjectFor MUTATES the engine while it builds a
// View (rules/layers.go appends to e.derivedPTFrames and defers a
// truncation, writes derivedDepth/scratch, and reuses the active()
// continuous-effect cache), so two projections of the same live engine
// running concurrently corrupt each other's in-progress layer-7 P/T frames.
//
// The three live-projection sites — Subscribe (host/session.go),
// onMatchStart and pushRewind (host/fanout.go, host/undo.go) — used to hold
// only m.mu.RLock while calling snapshotFrame/snapshotBody, which is a read
// lock over a write. They now go through Registry.projectLive, which takes
// m.mu EXCLUSIVELY.
//
// The test is lock-sensitive, not probabilistic: it holds m.mu.RLock (the
// lock the old projection used) and asserts that each production entry point
// BLOCKS instead of completing. With the fix reverted the entry point's own
// RLock is compatible, it completes, and the assertion fails. Each phase
// then releases the read lock and asserts the entry point completes and
// delivers a decodable frame (no deadlock), and a final storm runs many
// exclusive live projections concurrently to prove none corrupts a
// snapshot.

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/state"
)

// liveProjectionBoard plays fourSeatTable("t1", false) to completion once,
// publishes its finished match as the table's live match (so Subscribe's
// focus branch has something to project), and returns the registry, the
// table and that match. A finished match still holds a real, populated
// engine — the exact object snapshotBody projects — while the match
// goroutine is gone, so the only concurrent writers to the engine are this
// test's own projections.
func liveProjectionBoard(t *testing.T) (*Registry, *table, *match) {
	t.Helper()
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil {
		t.Fatal("finished table recorded no match")
	}
	// t.cur is normally cleared once a match finishes; publish it so
	// Subscribe's focus branch projects a live engine.
	tb.mu.Lock()
	tb.cur = m
	tb.mu.Unlock()
	return r, tb, m
}

// assertLiveProjectionPinned holds m.mu.RLock and asserts that run does not
// complete while it is held, then releases the lock and asserts run completes
// within a bound. The precondition it depends on is the new lock discipline:
// run must take m.mu exclusively (projectLive). Under the old RLock
// discipline the read lock is compatible, run finishes early, and the
// "completed while pinned" branch fails — a real regression, not a timing
// guess. A non-nil label appears in the failure so a run names the exact
// entry point that escaped exclusive ownership.
func assertLiveProjectionPinned(t *testing.T, m *match, label string, run func()) {
	t.Helper()
	resumed := make(chan struct{})
	go func() {
		run()
		close(resumed)
	}()
	// Hold the m.mu read lock — the lock the broken projection took — and
	// give run every chance to (wrongly) proceed.
	m.mu.RLock()
	for i := 0; i < 2000; i++ {
		runtime.Gosched()
	}
	select {
	case <-resumed:
		m.mu.RUnlock()
		t.Fatalf("precondition failed: %s completed while m.mu was pinned by a reader; a live projection must take m.mu exclusively", label)
	default:
	}
	m.mu.RUnlock()
	select {
	case <-resumed:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not complete after m.mu was released — deadlock", label)
	}
}

// TestConcurrentLiveSnapshotsSerializeEngineProjection proves every live
// engine projection takes exclusive m.mu, and that concurrent projections
// produce valid, uncorrupted snapshots.
func TestConcurrentLiveSnapshotsSerializeEngineProjection(t *testing.T) {
	t.Parallel()
	r, tb, m := liveProjectionBoard(t)

	// Precondition: the board is real and the view derives characteristics
	// for it. Without objects on the battlefield the projection would have
	// nothing whose layer-7 frames could be corrupted, and this test would
	// pass vacuously.
	board := 0
	for _, p := range m.e.G.Players {
		board += len(m.e.G.Zone(state.ZBattlefield, p.ID))
	}
	if board == 0 {
		t.Fatalf("precondition: finished match has no battlefield objects to project")
	}
	// A live projection in isolation succeeds and carries derived P/T on at
	// least one permanent, so the frames the race would corrupt are actually
	// built. This also pins the deterministic (single-threaded) result the
	// storm below must reproduce.
	var base protocol.Snapshot
	r.projectLive(m, func() { base = r.snapshotBody(tb, m) })
	derivedSeen := false
	for _, p := range base.View.Players {
		for _, c := range p.Battlefield {
			if c.Power != 0 || c.Toughness != 0 {
				derivedSeen = true
			}
		}
	}
	if !derivedSeen {
		t.Fatalf("precondition: projected board has no permanent with derived power/toughness")
	}

	// A focus subscriber must exist for onMatchStart/pushRewind to build a
	// snapshot at all (they return early with no sessions); Subscribe also
	// exercises the snapshot path itself.
	focus := r.OpenSession()
	if err := r.Subscribe(focus, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if f := <-focus.Out(); f.T != protocol.TSnapshot {
		t.Fatalf("focus Subscribe delivered %s first, want TSnapshot", f.T)
	}

	// Phase A: Subscribe's own live projection.
	assertLiveProjectionPinned(t, m, "Subscribe", func() {
		s := r.OpenSession()
		if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
			t.Errorf("Subscribe: %v", err)
			return
		}
		select {
		case f := <-s.Out():
			if f.T != protocol.TSnapshot {
				t.Errorf("concurrent Subscribe delivered %s, want TSnapshot", f.T)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("concurrent Subscribe delivered no snapshot")
		}
	})

	// Phase B: match-start's live projection.
	assertLiveProjectionPinned(t, m, "onMatchStart", func() {
		r.onMatchStart(tb, m)
	})

	// Phase C: rewind's live projection.
	assertLiveProjectionPinned(t, m, "pushRewind", func() {
		r.pushRewind(tb, m)
	})

	// Phase D: many exclusive live projections at once must all produce a
	// snapshot with the same head and a non-nil seat list as the
	// single-threaded baseline — a corrupted shared derived-P/T stack would
	// show up as a panicked goroutine (crashing the test binary) or a
	// divergent snapshot.
	const n = 32
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			var snap protocol.Snapshot
			var panicked any
			func() {
				defer func() { panicked = recover() }()
				r.projectLive(m, func() { snap = r.snapshotBody(tb, m) })
			}()
			if panicked != nil {
				errs <- fmt.Errorf("storm projection panicked: %v", panicked)
				return
			}
			if snap.Head != base.Head {
				errs <- fmt.Errorf("storm snapshot head %d differs from baseline %d", snap.Head, base.Head)
				return
			}
			if snap.Seats == nil {
				errs <- fmt.Errorf("storm snapshot has nil Seats")
				return
			}
			errs <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent live projection: %v", err)
		}
	}

	// The match mutex is not left held after the storm.
	if !m.mu.TryLock() {
		t.Fatal("m.mu still held after every live projection returned")
	}
	m.mu.Unlock()
}
