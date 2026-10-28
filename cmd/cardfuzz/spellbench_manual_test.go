package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSpellbenchTwinManualPayGollumAdventure pins the manual-pay position
// round di1 found: seed 10451216379200822465, priority decision 436, Gollum,
// Silent Slinker. The neutral world synthesized payment plans on a manual
// host, so its plan-reading bot preferred the plan-backed normal cast while
// the native bot, whose manual wire offers no plans, cast the adventure. The
// twin now puts both agents on manual mana, so both worlds ask the policy the
// same question and the decision agrees.
func TestSpellbenchTwinManualPayGollumAdventure(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	var job sbJob
	for _, j := range sbJobs(p, 1, 4) {
		if j.seed == 10451216379200822465 {
			job = j
			break
		}
	}
	if job.decks == nil {
		t.Fatalf("seed 1 has no game with the recorded seed 10451216379200822465")
	}
	g := playSBTwin(reg, job.decks, job.seed, sbOptions{
		mode: "twin", policy: "bot", autopay: false, maxSteps: 20000,
		maxTurns: 100, maxIntents: 20000, maxObjects: 20000, games: 1, seed: 1, workers: 1,
	})
	if g.err != "" {
		t.Fatalf("twin game failed: %s", g.err)
	}
	for _, f := range g.fails {
		if f.SB.Index == 436 {
			t.Fatalf("manual-pay decision 436 still disagrees: %s", f.Diag)
		}
	}
	if len(g.fails) != 0 {
		t.Fatalf("%d disagreements on the recorded manual-pay game", len(g.fails))
	}
}
