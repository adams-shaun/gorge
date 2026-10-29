package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwinSeed41TargetsResidueFixed pins the targeting/selection disagreements
// the held-out run seed 41 (-games 100, -autopay all) measured before
// sbp-twin-targets-s41c rebuilt the neutral world's target groups, hand moves
// and target-effect SVars:
//
//   - up-to-two targets: Unexplained Absence's TargetsForEachPlayer$ ask lost
//     its per-controller grouping, so the neutral seat finished after one pick
//     where the native seat took the other controller's target.
//   - frost bite: targetEffect could not evaluate NumDmg$ X
//     (Count$Compare Y GE3.3.2 over Count$Valid Permanent.Snow+YouCtrl), so
//     the neutral target choice differed.
//   - ugin's labyrinth: a hand-origin ChangeZone was rebuilt as the generic
//     exile arm, not gorge's deterministic hand_move first-Max take; the
//     imprint pick and the priority plan that follows it disagreed.
//
// The native pick is played, so the seed names the same game at every run; a
// decision the game no longer reaches is a broken precondition, not a pass.
func TestTwinSeed41TargetsResidueFixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 41, 100)
	cases := []struct {
		name  string
		seed  uint64
		kinds []string
		last  int
	}{
		{"up-to-two targets", 7577800171980809590, []string{"choose_target"}, 840},
		{"frost bite", 4276977129001154572, []string{"choose_target"}, 858},
		{"ugin's labyrinth", 844109554059487773, []string{"select_object", "priority"}, 107},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var job *sbJob
			for i := range jobs {
				if jobs[i].seed == tc.seed {
					job = &jobs[i]
				}
			}
			if job == nil {
				t.Fatalf("run seed 41 no longer contains job seed %d", tc.seed)
			}
			g := playSBTwin(reg, job.decks, job.seed, sbOptions{
				mode: "twin", policy: "bot", autopay: true,
				maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
			})
			if g.err != "" {
				t.Fatalf("game failed: %s", g.err)
			}
			if g.truncated {
				t.Fatal("game truncated before its decisions were compared; the test proves nothing")
			}
			total := 0
			for _, n := range g.tw.Total {
				total += n
			}
			if total <= tc.last {
				t.Fatalf("the game compared %d decisions and never reached decision %d; the test proves nothing", total, tc.last)
			}
			for _, kind := range tc.kinds {
				if g.tw.Total[kind] == 0 {
					t.Fatalf("no %s decision was compared; the test proves nothing", kind)
				}
			}
			for _, f := range g.fails {
				if f.SB == nil {
					continue
				}
				for _, kind := range tc.kinds {
					if f.SB.Kind == kind {
						t.Errorf("recorded %s disagreement is back: %s", kind, f.Diag)
					}
				}
			}
		})
	}
}
