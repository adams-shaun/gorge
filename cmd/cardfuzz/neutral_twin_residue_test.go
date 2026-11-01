package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinResidueFixed pins two native-vs-v2 decision disagreements at
// the positions they were recorded at:
//
//   - seed 8829294814793142954, decision 301 (engine_autopay) and 331/404
//     (manual): Corpse Harvester's EACH "Zombie and Swamp" search. The
//     neutral world must answer the same per-type selection, not finish it
//     after the first card.
//   - seed 445728488310520825, decision 3 in both modes: the seat's third
//     London keep/mulligan ask.
//
// The native pick is played, so the seed names the same game at every run;
// only the neutral answer is under test. A decision the game no longer
// reaches is a broken precondition, not a pass.
func TestNeutralTwinResidueFixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 2, 40)
	jobAt := func(seed uint64) sbJob {
		t.Helper()
		for _, j := range jobs {
			if j.seed == seed {
				return j
			}
		}
		t.Fatalf("run seed 2 no longer contains job seed %d", seed)
		return sbJob{}
	}
	cases := []struct {
		name    string
		seed    uint64
		autopay bool
		indices []int
		kind    string
	}{
		{"search/autopay", 8829294814793142954, true, []int{301}, "select_object"},
		{"search/manual", 8829294814793142954, false, []int{331, 404}, "select_object"},
		{"mulligan/autopay", 445728488310520825, true, []int{3}, "mulligan"},
		{"mulligan/manual", 445728488310520825, false, []int{3}, "mulligan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := jobAt(tc.seed)
			g := playSBTwin(reg, job.decks, job.seed, sbOptions{
				mode: "twin", policy: "bot", autopay: tc.autopay,
				maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
			})
			if g.err != "" {
				t.Fatalf("game failed: %s", g.err)
			}
			if g.tw.Total[tc.kind] == 0 {
				t.Fatalf("no %s decision was compared; the test proves nothing", tc.kind)
			}
			for _, f := range g.fails {
				if f.SB == nil {
					continue
				}
				for _, idx := range tc.indices {
					if f.SB.Index == idx {
						t.Fatalf("recorded disagreement at decision %d is back: %s", idx, f.Diag)
					}
				}
			}
		})
	}
}
