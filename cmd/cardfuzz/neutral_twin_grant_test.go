package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinGlorifierGrant pins the reported game
// 11045656273284599576 (job 79 of base seed 21): the native bot passes at the
// priority decisions where the neutral world re-activates Glorifier of Dusk's
// Flying pump for PayLife<2> while an identical grant from the same source is
// already pending. The neutral session kept only the latest grant per source,
// so it reported the second (Vigilance) as a duplicate and the first as not.
// The single-game job is the regression; the unit test in
// server/spellbench/neutral/pending_grant_test.go pins the mechanism without
// depending on the generated decks.
func TestNeutralTwinGlorifierGrant(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 21, 80)
	var job *sbJob
	for i := range jobs {
		if jobs[i].seed == 11045656273284599576 {
			job = &jobs[i]
			break
		}
	}
	if job == nil {
		t.Fatal("precondition: run seed 21 no longer contains job seed 11045656273284599576")
	}
	if job.idx != 79 {
		t.Fatalf("precondition: reported game is job %d, want 79", job.idx)
	}
	g := playSBTwin(reg, job.decks, job.seed, sbOptions{
		mode: "twin", policy: "bot", autopay: true,
		maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
	})
	if g.err != "" {
		t.Fatalf("game failed: %s", g.err)
	}
	if g.tw.Total["priority"] == 0 {
		t.Fatal("no priority decision was compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB == nil || f.Sig != "priority: pass -> activate_ability" {
			continue
		}
		if strings.Contains(f.Diag, "Glorifier of Dusk") {
			t.Fatalf("recorded disagreement at decision %d is back: %s", f.SB.Index, f.Diag)
		}
	}
}
