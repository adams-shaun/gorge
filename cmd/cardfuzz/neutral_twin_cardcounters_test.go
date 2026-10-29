package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinCardCountersTargetFixed pins the seed-45 twin's Shrine of
// Burning Rage target: the neutral world could not read Count$CardCounters,
// so its DealDamage carried no amount and the bot ranked Meria's Outrider
// (4/4) over Sneaking Guide (1/1), the only offered creature 3 damage kills.
func TestNeutralTwinCardCountersTargetFixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 45, 100)
	const seed = uint64(15168632577395293681)
	var job *sbJob
	for i := range jobs {
		if jobs[i].seed == seed {
			job = &jobs[i]
		}
	}
	if job == nil {
		t.Fatalf("run seed 45 no longer contains job seed %d", seed)
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
	if total <= 297 {
		t.Fatalf("the game compared %d decisions and never reached decision 297; the test proves nothing", total)
	}
	if g.tw.Total["choose_target"] == 0 {
		t.Fatal("no choose_target decision was compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB != nil && f.SB.Index == 297 && f.SB.Kind == "choose_target" {
			t.Fatalf("recorded choose_target disagreement at decision 297 is back: %s", f.Diag)
		}
	}
}
