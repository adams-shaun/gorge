package main

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNeutralTwinWallDefenderGrant pins the reported game
// 3174023599467939491 (job 6 of base seed 45): Wall of One Thousand Cuts
// activates its own CanAttackDefender grant, so the native seat attacks with
// it while the neutral declaration pool dropped it for its printed Defender.
// The single-game job is the regression; attack_grant_test.go pins the
// mechanism without depending on the generated decks.
func TestNeutralTwinWallDefenderGrant(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 45, 100)
	var job *sbJob
	for i := range jobs {
		if jobs[i].seed == 3174023599467939491 {
			job = &jobs[i]
			break
		}
	}
	if job == nil {
		t.Fatal("precondition: run seed 45 no longer contains job seed 3174023599467939491")
	}
	if job.idx != 6 {
		t.Fatalf("precondition: reported game is job %d, want 6", job.idx)
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
		if f.SB == nil || f.SB.Kind != "declare_attack" {
			continue
		}
		if f.SB.Index == 416 {
			t.Fatalf("recorded disagreement at decision %d is back: %s", f.SB.Index, f.Diag)
		}
	}
}
