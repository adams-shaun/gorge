package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Pins Farsight Ritual's partition in the game that produced the seed-45
// twin records at decisions 643 (Scroll Thief) and 644 (Island).
func TestTwinSeed45DigPartitionFixed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 45, 100)
	var job *sbJob
	for i := range jobs {
		if jobs[i].seed == 1756189677529149751 {
			job = &jobs[i]
		}
	}
	if job == nil {
		t.Fatalf("run seed 45 no longer contains game seed 1756189677529149751")
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
	if total <= 644 {
		t.Fatalf("only %d decisions compared; the game no longer reaches 643/644", total)
	}
	if g.tw.Total["arrange_card"] == 0 {
		t.Fatalf("no arrange_card decision compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB != nil && (f.SB.Index == 643 || f.SB.Index == 644) {
			t.Errorf("recorded disagreement at decision %d is back: %s", f.SB.Index, f.Diag)
		}
	}
}
