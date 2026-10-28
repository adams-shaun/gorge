package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinEntrancingLyre pins the recorded seed-9 Lyre position: the
// neutral world must map the native activation to the matching v2 candidate
// instead of passing. The native pick is played, so the seed names the same
// game; only the neutral answer is under test.
func TestNeutralTwinEntrancingLyre(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	const seed = uint64(15390265865825570874)
	var job sbJob
	found := false
	for _, j := range sbJobs(p, 9, 100) {
		if j.seed == seed {
			job, found = j, true
			break
		}
	}
	if !found {
		t.Fatalf("seed 9 no longer contains the recorded game seed %d", seed)
	}
	g := playSBTwin(reg, job.decks, job.seed, sbOptions{
		mode: "twin", policy: "bot", autopay: true,
		maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
	})
	// The recorded game hits a validator error after the Lyre position; the
	// decisions before it are still compared, so only the error-free prefix
	// matters.
	n := 0
	for _, c := range g.tw.Total {
		n += c
	}
	if n <= 131 {
		t.Fatalf("game stopped after %d decisions, before the recorded Lyre position 131 (err %q)", n, g.err)
	}
	for _, f := range g.fails {
		if f.SB != nil && f.SB.Index == 131 {
			t.Fatalf("recorded Lyre disagreement at decision 131 is back: %s", f.Diag)
		}
	}
}
