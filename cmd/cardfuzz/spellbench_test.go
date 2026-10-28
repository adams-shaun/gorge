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

func sbTestOptions(autopay bool) sbOptions {
	return sbOptions{mode: "twin", policy: "bot", autopay: autopay, maxSteps: 20000, maxTurns: 100, maxIntents: 20000,
		maxObjects: 20000, games: 4, seed: 1, workers: 2}
}

func sbTestRun(t *testing.T, autopay bool, failPath string) (*sbTwinSummary, string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sum, err := runSBTwin(reg, p, sbTestOptions(autopay), failPath, &out)
	if err != nil {
		t.Fatal(err)
	}
	return sum, out.String()
}

// TestSpellbenchTwinAgreesWithManualPay: with no engine auto-pay the neutral
// world reproduces the native pick on every compared decision, so the harness
// itself (deck injection, twin plumbing, tally) is exact.
func TestSpellbenchTwinAgreesWithManualPay(t *testing.T) {
	sum, out := sbTestRun(t, false, filepath.Join(t.TempDir(), "f.jsonl"))
	total := 0
	for _, n := range sum.Total {
		total += n
	}
	if total == 0 {
		t.Fatalf("no decision was compared:\n%s", out)
	}
	if sum.Disagreements != 0 || sum.Errors != 0 {
		t.Fatalf("%d disagreements, %d errors with -autopay off:\n%s", sum.Disagreements, sum.Errors, out)
	}
}

// TestSpellbenchTwinReportsAutopayDivergence: under engine_autopay the wire
// drops the engine's payment plan, so the twin must report classified
// disagreements, and a record's repro line must replay to the same decision.
func TestSpellbenchTwinReportsAutopayDivergence(t *testing.T) {
	fails := filepath.Join(t.TempDir(), "f.jsonl")
	sum, out := sbTestRun(t, true, fails)
	if sum.Disagreements == 0 {
		t.Fatalf("no disagreement under -autopay all (the payment hypothesis is falsified):\n%s", out)
	}
	if len(sum.Classes) == 0 || !strings.Contains(out, "divergence classes:") {
		t.Fatalf("disagreements are not classified:\n%s", out)
	}
	b, err := os.ReadFile(fails)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != sum.Disagreements {
		t.Fatalf("%d failure lines for %d disagreements", len(lines), sum.Disagreements)
	}
	var rec failure
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil || rec.SB == nil {
		t.Fatalf("first record %v is not a twin record: %v", rec, err)
	}
	if !json.Valid(rec.SB.Request) || len(rec.SB.Request) == 0 || rec.SB.Native == rec.SB.V2 {
		t.Fatalf("record lacks the v2 request or distinct picks: %+v", rec.SB)
	}
	reg := testutil.CorpusRegistry(t)
	var rep bytes.Buffer
	if code := runSBRepro(reg, rec, sbOptions{maxSteps: 20000}, &rep); code != 2 {
		t.Fatalf("repro exit %d, want 2:\n%s", code, rep.String())
	}
	if !strings.Contains(rep.String(), rec.Diag) {
		t.Fatalf("repro does not name the recorded decision %q:\n%s", rec.Diag, rep.String())
	}
}

// TestSpellbenchFuzzDeterministic: one seed gives a byte-identical summary,
// whatever the worker interleaving.
func TestSpellbenchFuzzDeterministic(t *testing.T) {
	_, a := sbTestRun(t, true, filepath.Join(t.TempDir(), "a.jsonl"))
	_, b := sbTestRun(t, true, filepath.Join(t.TempDir(), "b.jsonl"))
	if a != b {
		t.Fatalf("two runs at one seed differ:\n--- a\n%s\n--- b\n%s", a, b)
	}
}
