package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwinSeed41ResidueFixed pins the native-vs-v2 disagreements the held-out
// run seed 41 (-games 100, -autopay all) measured, at the decisions they were
// recorded at. Each class is a fact the neutral world rebuilt differently
// from gorge's view:
//
//   - hot soup: a card another seat owns but the viewer may play from exile
//     was rebuilt in its owner's hand, so the viewer's planner never saw it.
//   - gemstone: a Gemstone Caverns dealt into the rebuilt game's opening hand
//     posed an opening ask and the rebuild lost every payment plan.
//   - graveyard shovel: a Hidden$ ChangeZone from a public zone is a
//     hidden_pick, not an exile pick.
//   - ad nauseam: a RepeatOptional$ election is a plain choose with its
//     resume facts, not a may-ability.
//   - hearth kami: activations reversed while being announced (a Sacrifice
//     cost, no tap) stayed in the turn's activation count.
//
// The native pick is played, so the seed names the same game at every run; a
// decision the game no longer reaches is a broken precondition, not a pass.
func TestTwinSeed41ResidueFixed(t *testing.T) {
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
		{"hot soup", 1007571457455225457, []int{85}, "priority"},
		{"gemstone", 18111260664460382838, []int{175, 184, 224, 232, 274, 282, 291, 324, 332}, "priority"},
		{"graveyard shovel", 17341545280275747725, []int{383, 600, 650}, "select_object"},
		{"ad nauseam", 9021346642920790427, []int{364}, "choose_boolean"},
		{"hearth kami", 4276977129001154572, []int{109, 111, 117, 119, 130, 132, 157, 159, 163, 165, 176, 178, 207, 209, 221, 223}, "priority"},
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
			for _, f := range g.fails {
				for _, idx := range tc.indices {
					if f.SB != nil && f.SB.Index == idx {
						t.Fatalf("recorded disagreement at decision %d is back: %s", idx, f.Diag)
					}
				}
			}
		})
	}
}
