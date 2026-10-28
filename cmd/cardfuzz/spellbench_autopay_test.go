package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// TestSpellbenchTwinReportsAutopayDivergence keeps the failure-record and
// repro coverage the autopay work must not lose: a remaining divergence is
// classified, written as one line per disagreement, carries the v2 request and
// the two distinct picks, and -repro replays it. The Moonlight Bargain game
// (seed 13819372491320860226) still diverges under engine_autopay: the neutral
// shadow planner cannot rebuild the native cast plan, so the neutral seat
// casts where the native bot passes.
func TestSpellbenchTwinReportsAutopayDivergence(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 2, 20)
	var job sbJob
	found := false
	for _, j := range jobs {
		if j.seed == 13819372491320860226 {
			job, found = j, true
			break
		}
	}
	if !found {
		t.Fatalf("run seed 2 game set no longer contains the recorded Moonlight Bargain seed")
	}
	o := sbOptions{mode: "twin", policy: "bot", autopay: true, maxSteps: 20000, maxTurns: 100,
		maxIntents: 20000, maxObjects: 20000, games: 20, seed: 2, workers: 1}
	g := playSBTwin(reg, job.decks, job.seed, o)
	if g.err != "" {
		t.Fatalf("game failed: %s", g.err)
	}
	if len(g.fails) == 0 {
		t.Fatal("no disagreement in the recorded game; the failing record cannot be tested")
	}
	sum := newSBTwinSummary()
	sum.add(job.seed, g)
	var out bytes.Buffer
	sum.print(&out, o)
	if !strings.Contains(out.String(), "divergence classes:") {
		t.Fatalf("disagreements are not classified:\n%s", out.String())
	}

	fails := filepath.Join(t.TempDir(), "f.jsonl")
	var buf bytes.Buffer
	for _, f := range g.fails {
		b, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(fails, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(fails)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != len(g.fails) {
		t.Fatalf("%d failure lines for %d disagreements", len(lines), len(g.fails))
	}
	var rec failure
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil || rec.SB == nil {
		t.Fatalf("first record %v is not a twin record: %v", rec, err)
	}
	if !json.Valid(rec.SB.Request) || len(rec.SB.Request) == 0 || rec.SB.Native == rec.SB.V2 {
		t.Fatalf("record lacks the v2 request or distinct picks: %+v", rec.SB)
	}
	var rep bytes.Buffer
	if code := runSBRepro(reg, rec, sbOptions{maxSteps: 20000}, &rep); code != 2 {
		t.Fatalf("repro exit %d, want 2:\n%s", code, rep.String())
	}
	if !strings.Contains(rep.String(), rec.Diag) {
		t.Fatalf("repro does not name the recorded decision %q:\n%s", rec.Diag, rep.String())
	}
}
