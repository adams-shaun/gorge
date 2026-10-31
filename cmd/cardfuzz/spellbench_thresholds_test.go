package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
)

// TestSpellbenchMinAgreeFailsBelowThreshold: a kind at 98% over enough
// samples is below -min-agree 99.5, so it is named and marked FAIL in the
// summary; a kind with fewer than 50 samples is never gated, and threshold 0
// disables the check.
func TestSpellbenchMinAgreeFailsBelowThreshold(t *testing.T) {
	sum := newSBTwinSummary()
	sum.Total["priority"], sum.Agree["priority"] = 100, 98
	// 49 samples is below the floor: 0% agreement must not be gated.
	sum.Total["target"], sum.Agree["target"] = 49, 0
	sum.Total[twin.Floated] = 20

	kinds := sum.failingKinds(99.5)
	if len(kinds) != 1 || kinds[0] != "priority" {
		t.Fatalf("failingKinds(99.5) = %v, want [priority] (a 49-sample kind must not be gated)", kinds)
	}
	if got := sum.failingKinds(0); len(got) != 0 {
		t.Fatalf("failingKinds(0) = %v, want nil (0 disables the check)", got)
	}
	if got := sum.failingKinds(97.5); len(got) != 0 {
		t.Fatalf("failingKinds(97.5) = %v, want nil (98%% is above 97.5%%)", got)
	}

	var buf bytes.Buffer
	sum.print(&buf, sbOptions{policy: "bot", autopay: true, seed: 2, games: 1, minAgree: 99.5})
	out := buf.String()
	if !strings.Contains(out, "priority") {
		t.Fatalf("summary does not name priority:\n%s", out)
	}
	var priorityLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "priority") {
			priorityLine = line
		}
	}
	if !strings.Contains(priorityLine, "98/") || !strings.Contains(priorityLine, "FAIL") {
		t.Fatalf("priority line is not marked FAIL at 98%%:\n%s", out)
	}
	targetLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "target") {
			targetLine = line
		}
	}
	if strings.Contains(targetLine, "FAIL") {
		t.Fatalf("a 49-sample kind is marked FAIL:\n%s", out)
	}
}

// TestOutcomeParityPaired reads the gap and its interval from the discordant
// pairs alone, and gates on the interval and the bar together.
func TestOutcomeParityPaired(t *testing.T) {
	even := sbOutcomeParityOf(12, 12, 200, 0, 0)
	if !even.ok || even.gapPP != 0 {
		t.Fatalf("12/12 of 200: ok=%v gap %+.1f, want gap 0", even.ok, even.gapPP)
	}
	if even.fails(2) {
		t.Fatal("12/12 of 200 fails with no gap")
	}

	loss := sbOutcomeParityOf(18, 8, 200, 1, 0)
	if math.Abs(loss.gapPP+5) > 1e-9 {
		t.Fatalf("18/8 of 200: gap %+.2f, want -5.00", loss.gapPP)
	}
	if math.Abs(loss.halfPP-5) > 0.05 {
		t.Fatalf("18/8 of 200: half-width %.2f, want 5.0", loss.halfPP)
	}

	clear := sbOutcomeParityOf(146, 81, 1995, 5, 0)
	if !clear.fails(2) {
		t.Fatalf("146/81 of 1995 (%+.2f +/- %.2f) does not fail -max-outcome-gap 2", clear.gapPP, clear.halfPP)
	}
	if clear.fails(4) {
		t.Fatalf("146/81 of 1995 (%+.2f) fails -max-outcome-gap 4", clear.gapPP)
	}
	if clear.fails(0) {
		t.Fatal("146/81 of 1995 fails -max-outcome-gap 0 (disabled)")
	}

	// A gap over the bar whose interval still includes zero is not a failure.
	if wide := sbOutcomeParityOf(5, 1, 100, 0, 0); wide.fails(2) {
		t.Fatalf("5/1 of 100 (%+.2f +/- %.2f) fails with an interval that includes 0", wide.gapPP, wide.halfPP)
	}

	if none := sbOutcomeParityOf(0, 0, 0, 3, 0); none.ok || none.fails(2) {
		t.Fatalf("no finished pair: ok=%v fails=%v, want neither", none.ok, none.fails(2))
	}
}

// TestOutcomeParityFailsOnForfeit: one forfeit fails at any bar, even with no
// gap and with the gap check off.
func TestOutcomeParityFailsOnForfeit(t *testing.T) {
	p := sbOutcomeParityOf(12, 12, 200, 0, 1)
	if !p.fails(2) || !p.fails(0) {
		t.Fatalf("a forfeit passes: fails(2)=%v fails(0)=%v", p.fails(2), p.fails(0))
	}
}
