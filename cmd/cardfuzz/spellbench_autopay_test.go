package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSpellbenchTwinAutopayMapsNativePick pins the example position from
// sbp-twin-autopay-pass: seed 13757245211066428519 (run seed 2, game 1),
// priority choose index 258. The native bot wants to float mana by hand
// there (the native decision offers an "activate" option), but an
// engine_autopay wire never offers a manual tap. Asked the full native
// decision the adapter picked the activation, Pick could not name it, and
// the twin reported candidate 0 (pass) against the neutral world's
// play_land. The view agent must instead ask the bot only the actions the
// wire offers, so the pick is the matching land candidate and the decision
// agrees.
func TestSpellbenchTwinAutopayMapsNativePick(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 2, 2)
	if len(jobs) < 2 || jobs[1].seed != 13757245211066428519 {
		t.Fatalf("game 1 of seed 2 is %v, want the recorded example seed", jobs)
	}
	g := playSBTwin(reg, jobs[1].decks, jobs[1].seed, sbOptions{
		mode: "twin", policy: "bot", autopay: true, maxSteps: 20000,
		maxTurns: 100, maxIntents: 20000, maxObjects: 20000, workers: 1,
	})
	if g.err != "" {
		t.Fatalf("example game failed: %s", g.err)
	}
	if g.tw.Total["priority"] == 0 {
		t.Fatal("no priority decision was compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB == nil {
			continue
		}
		if f.SB.Index == 258 {
			t.Fatalf("decision 258: native pick %d (%s) vs neutral pick %d (%s), want the offered land candidate",
				f.SB.Native, f.SB.NativeSem, f.SB.V2, f.SB.V2Sem)
		}
		if f.Sig == "priority: pass -> play_land" {
			t.Fatalf("a manual activation was projected to pass: %s", f.Diag)
		}
	}
	if g.tw.Agree["priority"] != g.tw.Total["priority"] {
		t.Fatalf("priority agreement %d/%d in the example game, want all; failures: %v",
			g.tw.Agree["priority"], g.tw.Total["priority"], g.fails)
	}
}
