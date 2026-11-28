package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// cleanCast is a clean game whose coverage casts card n times.
func cleanCast(card string, n int) played {
	cov := oracle.NewCoverage()
	cov.Cards[card] = &oracle.CardCoverage{Cast: n, Used: map[string]int{}}
	cov.Games = 1
	return played{res: oracle.Result{Winner: 0}, cov: cov}
}

// A run split by a hang writes -evidence counted over every segment's games:
// the segment that hung hands its coverage to the continuation, which merges it
// before writing, so a card short of oracle.EvidenceMinPlays in each segment
// alone but over it across the run is evidenced (and the -coverage file counts
// both segments' games).
func TestContinuationKeepsEarlierSegmentEvidence(t *testing.T) {
	const card = "Split Card"
	per := oracle.EvidenceMinPlays - 1 // short in each segment, enough over both
	if per < 1 || 2*per < oracle.EvidenceMinPlays {
		t.Fatalf("EvidenceMinPlays %d: no per-segment count is short alone and enough twice", oracle.EvidenceMinPlays)
	}
	dir := t.TempDir()
	ev, covFile := filepath.Join(dir, "run.evidence"), filepath.Join(dir, "run.coverage")
	oldEv, oldCov := evidencePath, coveragePath
	evidencePath, coveragePath = ev, covFile
	t.Cleanup(func() { evidencePath, coveragePath = oldEv, oldCov })
	t.Setenv(carryCoverageEnv, "")
	t.Cleanup(func() { segmentCoverage = nil })
	var continued bool
	saved := runContinuation
	runContinuation = func(int, int) int { continued = true; return 1 }
	t.Cleanup(func() { runContinuation = saved })
	o := sweepOpts{lead: "gorge", surface: "autopay", timeout: time.Second, seats: 2, refused: map[string]bool{}}

	// Segment 1: one clean game, then a hang; pair 2 is left to the continuation.
	ps := testPairs(3)
	seg1 := []played{cleanCast(card, per), {hang: true}, {}}
	if code, stdout := capture(t, func() int { return finishSweep("fdn", ps, seg1, 2, oracle.RunConfig{}, o, nil) }); code != 1 || !continued {
		t.Fatalf("segment 1: exit %d, continued %v, want 1 and a continuation\n%s", code, continued, stdout)
	}
	if rows := evidenceRows(t, ev); slices.Contains(rows, card) {
		t.Fatalf("segment 1 alone evidences %q with %d plays (rows %q): the test cannot tell a merge from one segment", card, per, rows)
	}
	carry := os.Getenv(carryCoverageEnv)
	if carry == "" {
		t.Fatalf("segment 1 handed the continuation no coverage (%s unset)", carryCoverageEnv)
	}

	// Segment 2, the continuation image: pair 2 alone, clean.
	if code, stdout := capture(t, func() int { return report("fdn", ps[2:], []played{cleanCast(card, per)}, oracle.RunConfig{}, o, nil) }); code != 0 {
		t.Fatalf("segment 2: exit %d\n%s", code, stdout)
	}
	if rows := evidenceRows(t, ev); !slices.Contains(rows, card) {
		t.Errorf("the continuation's evidence %q lacks %q (%d plays over both segments, %d needed)", rows, card, 2*per, oracle.EvidenceMinPlays)
	}
	if want := fmt.Sprintf("%s\t%d\t", card, 2*per); !slices.ContainsFunc(evidenceRows(t, covFile), func(r string) bool { return strings.HasPrefix(r, want) }) {
		t.Errorf("the coverage file has no %q row (cast over both segments): %q", want, evidenceRows(t, covFile))
	}
	if _, err := os.Stat(carry); !os.IsNotExist(err) {
		t.Errorf("the carried coverage file %s outlives the continuation that merged it (stat: %v)", carry, err)
	}
}
