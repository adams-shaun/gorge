package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwinSeed41CombatResidueFixed pins the combat disagreements the held-out
// run seed 41 (-games 100, -autopay all) measured before the neutral rebuild
// was corrected, at the decisions they were recorded at:
//
//   - devil token: the neutral declined the block the native declared because
//     the engine's offer honoured an attacker-hosted CantBlockBy static.
//   - plains block: the native kept a face-down manifested Plains home while
//     the neutral blocked with it: the printed type line said land, but CR
//     708.2 makes a face-down permanent a 2/2 creature.
//   - face-down attacks: the same face-down creature test, in the other
//     declaration, plus the group's unit order being the engine's option
//     order rather than current-unit-first.
//   - hyena pack: a Guardians of Meletis carrying a hidden "can't attack or
//     block" line was in the neutral's board-derived blocker set but not the
//     engine's offer.
//
// The native pick is played, so the seed names the same game at every run; a
// decision the game no longer reaches is a broken precondition, not a pass.
func TestTwinSeed41CombatResidueFixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 41, 100)
	cases := []struct {
		name    string
		seed    uint64
		indices []int
		kind    string
	}{
		{"devil token", 4276977129001154572, []int{541}, "declare_block"},
		{"plains block", 5640536404233877349, []int{406}, "declare_block"},
		{"face-down attacks", 5640536404233877349, []int{559, 656, 657, 659, 660}, "declare_attack"},
		{"hyena pack", 1073112203691670766, []int{483}, "declare_block"},
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
			if last := tc.indices[len(tc.indices)-1]; total <= last {
				t.Fatalf("the game compared %d decisions and never reached decision %d; the test proves nothing", total, last)
			}
			if g.tw.Total[tc.kind] == 0 {
				t.Fatalf("no %s decision was compared; the test proves nothing", tc.kind)
			}
			var back []int
			for _, f := range g.fails {
				if f.SB == nil {
					continue
				}
				for _, idx := range tc.indices {
					if f.SB.Index == idx {
						back = append(back, idx)
					}
				}
			}
			if len(back) > 0 {
				t.Fatalf("recorded disagreements are back at decisions %v", back)
			}
		})
	}
}
