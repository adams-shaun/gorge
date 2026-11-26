package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// copyExileRiderRun plays a Dominion Infiltration scenario and returns the
// most copies of source on the stack at once, the most Grizzly Bears tokens
// ever on the battlefield (the copies' entry precondition), and the number of
// Grizzly Bears tokens left in the last snapshot.
func copyExileRiderRun(t *testing.T, scenario, source string) (peak, maxTokens, finalTokens int) {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	for _, s := range res.Snapshots {
		n := 0
		for _, e := range s.Stack {
			if e.Kind == "ability" && strings.Contains(e.Source, source) {
				n++
			}
		}
		peak = max(peak, n)
		tokens := 0
		for _, p := range s.Permanents {
			if p.Name == "Grizzly Bears" && p.Token {
				tokens++
			}
		}
		maxTokens = max(maxTokens, tokens)
		finalTokens = tokens
	}
	return peak, maxTokens, finalTokens
}

// CR 603.7a: even with X = 0 (no target, no copy) Dominion Infiltration's
// resolving instruction still creates the delayed "exile those tokens" ability.
// It goes on the stack at the next end step and resolves doing nothing.
func TestCopyPermanentExileRiderZeroTargetsStillTriggers(t *testing.T) {
	scenario := `{"name":"dominion-empty-exile","cr":["603.7a"],"why":"zero copies still create the delayed exile trigger",
 "setup":{"p0":{"battlefield":["Mountain"],"hand":["Dominion Infiltration"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Dominion Infiltration","mana":"RRR","answers":[{"kind":"choose","pick":["X = 0"]}]},{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}]}`
	peak, copies, left := copyExileRiderRun(t, scenario, "Dominion Infiltration")
	if copies != 0 {
		t.Fatalf("%d Grizzly Bears copies made at X = 0, want 0", copies)
	}
	if peak != 1 {
		t.Fatalf("%d Dominion Infiltration triggers on the stack at once, want 1", peak)
	}
	if left != 0 {
		t.Fatalf("%d Grizzly Bears copies left, want 0 (the trigger exiles nothing)", left)
	}
}

// CR 603.7a: X = 2 makes two copies and ONE delayed ability exiles both, not
// one trigger per copy.
func TestCopyPermanentExileRiderOverSeveralCopiesIsOneTrigger(t *testing.T) {
	scenario := `{"name":"dominion-two-exile","cr":["603.7a"],"why":"one delayed exile trigger for both copies",
 "setup":{"p0":{"battlefield":["Mountain"],"hand":["Dominion Infiltration"]},"p1":{"battlefield":["Grizzly Bears","Grizzly Bears"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Dominion Infiltration","mana":"RRRRR","targets":["p1:Grizzly Bears","p1:Grizzly Bears#2"],"answers":[{"kind":"choose","pick":["X = 2"]}]},{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}]}`
	peak, copies, left := copyExileRiderRun(t, scenario, "Dominion Infiltration")
	if copies != 2 {
		t.Fatalf("%d Grizzly Bears copies made at X = 2, want 2", copies)
	}
	if peak != 1 {
		t.Fatalf("%d Dominion Infiltration triggers on the stack at once, want 1", peak)
	}
	if left != 0 {
		t.Fatalf("%d Grizzly Bears copies left, want 0 (the one trigger exiles both)", left)
	}
}
