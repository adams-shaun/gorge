package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
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

// TestSpellbenchSummaryExcludesChosenColourSkips: ChosenColour is a skip class
// like Floated and Opaque, so it must not sit in the agreement denominator or
// appear as a decision kind, and its count belongs in the skip sentence.
func TestSpellbenchSummaryExcludesChosenColourSkips(t *testing.T) {
	sum := newSBTwinSummary()
	sum.Total["priority"], sum.Agree["priority"] = 10, 10
	sum.Total[twin.ChosenColour] = 90
	sum.Total[twin.Floated] = 3
	sum.Total[twin.Opaque] = 2
	var buf bytes.Buffer
	sum.print(&buf, sbOptions{policy: "bot", autopay: true, seed: 2, games: 1})
	out := buf.String()
	if !strings.Contains(out, "agreement 10/10 ") {
		t.Fatalf("ChosenColour is in the agreement denominator:\n%s", out)
	}
	if !strings.Contains(out, "90 where a chosen-colour source was in play") {
		t.Fatalf("the skip sentence does not report the ChosenColour count:\n%s", out)
	}
	if strings.Contains(out, twin.ChosenColour) {
		t.Fatalf("ChosenColour is listed as a decision kind:\n%s", out)
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
