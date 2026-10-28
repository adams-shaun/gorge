package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
)

// rebuildFailLine returns the summary line naming the rebuild_fail criterion.
func rebuildFailLine(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "rebuild_fail") {
			return line
		}
	}
	return ""
}

// TestSpellbenchFailReasonNamed: a 100%-agreement tally with one rebuild
// failure in the 5,865 compared decisions the smoke run reported passes
// -min-agree 99.5 under the default 0.5% rebuild budget; with the budget at 0
// the same tally fails and the FAIL line names rebuild_fail.
func TestSpellbenchFailReasonNamed(t *testing.T) {
	sum := newSBTwinSummary()
	sum.Total["priority"], sum.Agree["priority"] = 5865, 5865
	sum.Total[twin.Opaque] = 1

	// Preconditions: the rebuild failure is where the budget rule looks, the
	// compared count is the run's 5,865, and the decision kind is at 100%.
	if got := sum.Total[twin.Opaque]; got != 1 {
		t.Fatalf("precondition: Total[Opaque] = %d, want 1", got)
	}
	if got := sum.compared(); got != 5865 {
		t.Fatalf("precondition: compared() = %d, want 5865", got)
	}
	if sum.Agree["priority"] != sum.Total["priority"] {
		t.Fatalf("precondition: priority is not at 100%% agreement")
	}

	const minAgree = 99.5
	if reasons := sum.failReasons(minAgree, 0.5); len(reasons) != 0 {
		t.Fatalf("one rebuild failure in 5,865 compared decisions fails the default 0.5%% budget: %v", reasons)
	}

	// The budget is load-bearing: 1/5,866 is below 0.5% but above zero.
	pct := sum.rebuildFailPct()
	if pct <= 0 || pct >= 0.5 {
		t.Fatalf("precondition: rebuildFailPct() = %v, want 0 < pct < 0.5", pct)
	}

	reasons := sum.failReasons(minAgree, 0)
	if len(reasons) != 1 || !strings.Contains(reasons[0], "rebuild_fail") {
		t.Fatalf("failReasons(99.5, 0) = %v, want one reason naming rebuild_fail", reasons)
	}

	var pass bytes.Buffer
	sum.print(&pass, sbOptions{policy: "bot", autopay: true, seed: 1, games: 20, minAgree: minAgree, maxRebuildFail: 0.5})
	if line := rebuildFailLine(pass.String()); line == "" || strings.Contains(line, "FAIL") {
		t.Fatalf("summary at budget 0.5%% should name rebuild_fail and not mark it FAIL:\n%s", pass.String())
	}

	var fail bytes.Buffer
	sum.print(&fail, sbOptions{policy: "bot", autopay: true, seed: 1, games: 20, minAgree: minAgree, maxRebuildFail: 0})
	line := rebuildFailLine(fail.String())
	if line == "" {
		t.Fatalf("summary at budget 0 does not name rebuild_fail:\n%s", fail.String())
	}
	if !strings.Contains(line, "FAIL") {
		t.Fatalf("summary at budget 0 does not mark rebuild_fail FAIL:\n%s", fail.String())
	}

	// Every failed criterion is named: a harness error is the other run-level
	// criterion, and its summary line is marked FAIL too.
	errSum := newSBTwinSummary()
	errSum.Total["priority"], errSum.Agree["priority"] = 100, 100
	errSum.Errors, errSum.ErrorText["validator: V7"] = 1, 1
	if reasons := errSum.failReasons(minAgree, 0.5); len(reasons) != 1 || reasons[0] != "errors 1" {
		t.Fatalf("failReasons names a harness error as %v, want [errors 1]", reasons)
	}
	var errOut bytes.Buffer
	errSum.print(&errOut, sbOptions{policy: "bot", autopay: true, seed: 1, games: 1, minAgree: minAgree, maxRebuildFail: 0.5})
	errLine := ""
	for _, l := range strings.Split(errOut.String(), "\n") {
		if strings.Contains(l, "error validator") {
			errLine = l
		}
	}
	if errLine == "" || !strings.Contains(errLine, "FAIL") {
		t.Fatalf("summary does not mark the harness error FAIL:\n%s", errOut.String())
	}
}
