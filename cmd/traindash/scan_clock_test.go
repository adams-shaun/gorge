package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestScanUsesInjectedClock pins the dashboard clock boundary that the
// internal/archtest exemption for cmd/traindash rests on (arch_test.go: the
// clock serves only dashboard metadata and the live/stale status, never
// engine behaviour). It asserts that Snapshot.Generated is exactly the
// scanner's injected Now — never the wall clock — and that the same
// injected clock drives the live/stale classification.
func TestScanUsesInjectedClock(t *testing.T) {
	fi, err := os.Stat(filepath.Join(fixtureRoot, "expa/runs/arm1.stderr"))
	if err != nil {
		t.Fatal(err)
	}
	base := fi.ModTime()

	// Precondition: the injected clock sits far behind the wall clock, so
	// the Generated assertion below is not vacuous — it would fail if Scan
	// silently read time.Now instead of s.Now.
	fixed := base.Add(-24 * time.Hour)
	if d := time.Since(fixed); d < 23*time.Hour {
		t.Fatalf("precondition: injected clock %v is only %v behind the wall clock", fixed, d)
	}

	s := NewScanner([]string{fixtureRoot})
	s.Now = func() time.Time { return fixed }
	snap := s.Scan()
	if !snap.Generated.Equal(fixed) {
		t.Fatalf("Snapshot.Generated = %v, want the injected clock %v exactly", snap.Generated, fixed)
	}

	// The same injected clock drives the live/stale boundary. arm1's stderr
	// ends in "..." (a stage that never finished), so it is running inside
	// liveWindow (5m) of its mtime and stale just past it — both measured
	// from the injected time, not the wall clock.
	s.Now = func() time.Time { return base.Add(4 * time.Minute) }
	if r := findRun(t, s.Scan(), "expa/runs/arm1"); r.Status != "running" {
		t.Fatalf("arm1 at injected clock+4m: status %q, want running", r.Status)
	}
	s.Now = func() time.Time { return base.Add(liveWindow + time.Minute) }
	if r := findRun(t, s.Scan(), "expa/runs/arm1"); r.Status != "stale" {
		t.Fatalf("arm1 at injected clock+6m: status %q, want stale", r.Status)
	}
}
