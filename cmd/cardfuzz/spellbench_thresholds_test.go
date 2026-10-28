package main

import (
	"bytes"
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

// TestSpellbenchOutcomeGapThreshold: a 20 pp gap with an interval that
// excludes zero fails -max-outcome-gap 10; a 2 pp gap whose interval includes
// zero passes, and 0 disables the check.
func TestSpellbenchOutcomeGapThreshold(t *testing.T) {
	wide := sbOutcomeParityOf(40, 100, 60, 100)
	if !wide.ok {
		t.Fatal("40/100 vs 60/100: parity not computed")
	}
	if wide.loPP <= 0 && wide.hiPP >= 0 {
		t.Fatalf("40/100 vs 60/100: CI [%.1f, %.1f] includes 0; the gap-exclusion precondition is not met", wide.loPP, wide.hiPP)
	}
	if !wide.fails(10) {
		t.Fatalf("40/100 vs 60/100 (gap %+.1f pp, CI [%.1f, %.1f]) does not fail -max-outcome-gap 10", wide.gapPP, wide.loPP, wide.hiPP)
	}
	if wide.fails(25) {
		t.Fatalf("40/100 vs 60/100 (gap %+.1f pp) fails -max-outcome-gap 25", wide.gapPP)
	}
	if wide.fails(0) {
		t.Fatalf("40/100 vs 60/100 fails -max-outcome-gap 0 (disabled)")
	}

	close := sbOutcomeParityOf(50, 100, 48, 100)
	if !close.ok {
		t.Fatal("50/100 vs 48/100: parity not computed")
	}
	if close.loPP > 0 || close.hiPP < 0 {
		t.Fatalf("50/100 vs 48/100: CI [%.1f, %.1f] excludes 0; the pass case must be a zero-including interval", close.loPP, close.hiPP)
	}
	if close.fails(10) {
		t.Fatalf("50/100 vs 48/100 (gap %+.1f pp, CI [%.1f, %.1f]) fails -max-outcome-gap 10", close.gapPP, close.loPP, close.hiPP)
	}
}
