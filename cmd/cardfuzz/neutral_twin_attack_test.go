package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwinAttackBarredByOtherPermanent pins game 6078429791474061835 of base
// seed 71: Doomed Artisan makes its controller's Sculpture tokens unable to
// attack, and the neutral world's whole-declaration pool, which is read off
// the board, kept them. A token in the pool pushes a real attacker out of a
// declaration whose size the host fixes, so the seat planned a different
// attack.
func TestTwinAttackBarredByOtherPermanent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	var job *sbJob
	jobs := sbJobs(p, 71, 100)
	for i := range jobs {
		if jobs[i].seed == 6078429791474061835 {
			job = &jobs[i]
			break
		}
	}
	if job == nil {
		t.Fatal("precondition: run seed 71 no longer contains game 6078429791474061835")
	}
	g := playSBTwin(reg, job.decks, job.seed, sbOptions{
		mode: "twin", policy: "bot", autopay: true,
		maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
	})
	if g.err != "" {
		t.Fatalf("game failed: %s", g.err)
	}
	if g.tw.Total["declare_attack"] == 0 {
		t.Fatal("no declare_attack decision was compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB != nil && f.SB.Kind == "declare_attack" {
			t.Fatalf("declare_attack disagreement at decision %d: %s", f.SB.Index, f.Diag)
		}
	}
}
