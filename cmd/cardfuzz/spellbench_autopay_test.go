package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
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

// TestSpellbenchTwinArmsChosenColourSkip: playSBTwin must give the Twin the
// card registry its ChosenColour skip needs. In the Moonlight Bargain game
// (seed 13819372491320860226) Temple of the Dragon Queen makes a colour chosen
// as it entered, which the v2 observation does not carry. Without the registry
// the skip never fires and two priority decisions are reported as
// "pass -> cast_spell"; with it they are counted as ChosenColour and the game
// has no priority disagreement at all.
func TestSpellbenchTwinArmsChosenColourSkip(t *testing.T) {
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
	if g.tw.Total["priority"] == 0 {
		t.Fatal("no priority decision was compared; the test proves nothing")
	}
	t.Logf("seed %d: chosen=%d priority=%d/%d fails=%d", job.seed, g.tw.Total[twin.ChosenColour], g.tw.Agree["priority"], g.tw.Total["priority"], len(g.fails))
	if g.tw.Total[twin.ChosenColour] == 0 {
		t.Fatalf("the ChosenColour skip never fired; the Twin has no registry")
	}
	for _, f := range g.fails {
		if f.Sig == "priority: pass -> cast_spell" {
			t.Fatalf("priority decision %d was not skipped as ChosenColour: %s", f.SB.Index, f.Diag)
		}
	}
}

// TestSpellbenchTwinReportsAutopayDivergence keeps the failure-record and
// repro coverage the autopay work must not lose: a remaining divergence is
// classified, written as one line per disagreement, carries the v2 request and
// the two distinct picks, and -repro replays it. The Benthic Giant game (seed
// 8507278219836860430) is the fixture: Benthic Giant is in hand under Absorb
// Energy's perpetual {1} cost grant, which the v2 observation cannot express,
// so the neutral world prices the printed cost and picks Tidal Barracuda. The
// native 4U plan is correct; the Moonlight Bargain game is now a ChosenColour
// skip, not a divergence.
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
		if j.seed == 8507278219836860430 {
			job, found = j, true
			break
		}
	}
	if !found {
		t.Fatalf("run seed 2 game set no longer contains the recorded Benthic Giant seed")
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
	var benthic *failure
	for _, f := range g.fails {
		if f.SB == nil {
			continue
		}
		if strings.Contains(string(f.SB.NativeSem), "Benthic Giant") &&
			strings.Contains(string(f.SB.V2Sem), "Tidal Barracuda") {
			benthic = f
			break
		}
	}
	if benthic == nil {
		t.Fatalf("no Benthic Giant/Tidal Barracuda record in the game; failures: %v", g.fails)
	}
	if benthic.Sig != "priority: cast_spell -> cast_spell" {
		t.Fatalf("Benthic Giant record has signature %q, want priority: cast_spell -> cast_spell", benthic.Sig)
	}
	t.Logf("benthic: fails=%d index=%d sig=%s", len(g.fails), benthic.SB.Index, benthic.Sig)
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
	for _, line := range lines {
		var r failure
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("failure line is not a record: %v", err)
		}
		if r.SB != nil && r.SB.Index == benthic.SB.Index && r.Sig == benthic.Sig {
			rec = r
			break
		}
	}
	if rec.SB == nil {
		t.Fatalf("the Benthic Giant record did not round-trip through the failure file")
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
