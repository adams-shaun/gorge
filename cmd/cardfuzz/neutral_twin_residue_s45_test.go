package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinResidueS45Fixed pins the seven native-vs-v2 disagreements
// the held-out run seed 45 (-games 100, -autopay all) measured, at the
// decisions they were recorded at:
//
//   - seed 8100850647569168289 decision 242: Drowner Initiate is required to
//     attack by an attached Coercive Impetus's continuous Goad$ True static.
//   - seed 16436663646388306076 decision 617: Valley Questcaller's attached
//     Cessation forbids it attacking at all.
//   - seeds 11010936823729913063 decisions 62/146 and
//     1006105367537104573 decision 253: the neutral offered casts the native
//     priority did not (Heart of Kiran's crew-scoped AlternativeCost, and the
//     land-type grant the shadow's missing NameUniverse left unpaid).
//   - seed 12359012004085225572 decisions 286/287: The Sea Devils' third
//     chapter deals the triggering Salamander's combat damage to the target.
//
// The native pick is played, so the seed names the same game at every run; a
// decision the game no longer reaches is a broken precondition, not a pass.
func TestNeutralTwinResidueS45Fixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 45, 100)
	cases := []struct {
		name    string
		seed    uint64
		indices []int
		kind    string
	}{
		{"heart of kiran", 11010936823729913063, []int{62, 146}, "priority"},
		{"malach", 1006105367537104573, []int{253}, "priority"},
		{"elrond", 8100850647569168289, []int{242}, "declare_attack"},
		{"loxodon", 16436663646388306076, []int{617}, "declare_attack"},
		{"sea devils", 12359012004085225572, []int{286, 287}, "choose_target"},
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
				t.Fatalf("run seed 45 no longer contains job seed %d", tc.seed)
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
