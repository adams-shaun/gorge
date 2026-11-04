package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// atEOTStackPeak plays a scenario and returns the most abilities of the named
// source on the stack at once and the permanents of the given name left in the
// last snapshot.
func atEOTStackPeak(t *testing.T, scenario, source, perm string) (peak, left int) {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	for _, s := range res.Snapshots {
		n := 0
		for _, e := range s.Stack {
			if e.Kind == "ability" && strings.Contains(e.Source, source) {
				n++
			}
		}
		if testing.Verbose() {
			var st []string
			for _, e := range s.Stack {
				st = append(st, e.Kind+":"+e.Source)
			}
			t.Logf("step=%v stack=%v perms=%d", s.Step, st, len(s.Permanents))
		}
		peak = max(peak, n)
		left = 0
		for _, p := range s.Permanents {
			if p.Name == perm {
				left++
			}
		}
	}
	return peak, left
}

func passes(n int) string {
	return strings.Repeat(`,{"op":"pass","seat":0},{"op":"pass","seat":1}`, n)
}

// "Sacrifice those creatures at the beginning of the next end step" over
// several creatures is one delayed triggered ability (CR 603.7): Tears of Rage
// puts one trigger on the stack and it sacrifices both attackers.
func TestPumpAllSacrificeRiderIsOneTrigger(t *testing.T) {
	scenario := `{"name":"tears-one-trigger","cr":["603.7"],"why":"one delayed trigger for the group",
 "setup":{"p0":{"battlefield":["Mountain","Grizzly Bears","Grizzly Bears"],"hand":["Tears of Rage"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"attack","seat":0,"attackers":["p0:Grizzly Bears","p0:Grizzly Bears#2"],"defender":"p1"},
 {"op":"cast","seat":0,"card":"p0:Tears of Rage","mana":"CCRR"}` + passes(8) + `]}`
	peak, left := atEOTStackPeak(t, scenario, "Tears of Rage", "Grizzly Bears")
	if peak != 1 {
		t.Fatalf("%d Tears of Rage triggers on the stack at once, want 1", peak)
	}
	if left != 0 {
		t.Fatalf("%d Grizzly Bears left, want 0 (the one trigger sacrifices both)", left)
	}
}

// Return-all-and-sacrifice-them is likewise one trigger (CR 603.7).
func TestChangeZoneAllSacrificeRiderIsOneTrigger(t *testing.T) {
	scenario := `{"name":"push-one-trigger","cr":["603.7"],"why":"one delayed trigger for the group",
 "setup":{"p0":{"battlefield":["Mountain"],"hand":["Push the Limit"],"graveyard":["Smuggler's Copter","Aetherflux Car"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Push the Limit","mana":"CCCCCRR"}` + passes(1) + `,{"op":"attack","seat":0,"attackers":[],"defender":"p1"}` + passes(5) + `]}`
	peak, _ := atEOTStackPeak(t, scenario, "Push the Limit", "Smuggler's Copter")
	if peak != 1 {
		t.Fatalf("%d Push the Limit triggers on the stack at once, want 1", peak)
	}
}

// Foggy Swamp Visions with X = 0 exiles and copies nothing, but its
// "sacrifice those tokens" delayed trigger is still created and goes on the
// stack at the end step (CR 603.7).
func TestEmptyRememberedCopyRiderStillTriggers(t *testing.T) {
	scenario := `{"name":"foggy-empty","cr":["603.7"],"why":"an empty group trigger is still put on the stack",
 "setup":{"p0":{"battlefield":["Swamp","Swamp","Swamp"],"hand":["Foggy Swamp Visions"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Foggy Swamp Visions","mana":"CBB","answers":[{"kind":"choose","pick":["X = 0"]}]}` + passes(8) + `]}`
	peak, _ := atEOTStackPeak(t, scenario, "Foggy Swamp Visions", "Swamp")
	if peak != 1 {
		t.Fatalf("%d Foggy Swamp Visions triggers on the stack at once, want 1", peak)
	}
}

// Devastating Onslaught for X = 2 makes two copies and one trigger sacrifices
// both (CR 603.7).
func TestCopyPermanentSacrificeRiderOverSeveralCopiesIsOneTrigger(t *testing.T) {
	scenario := `{"name":"onslaught-two","cr":["603.7"],"why":"one delayed trigger for both copies",
 "setup":{"p0":{"battlefield":["Mountain","Grizzly Bears"],"hand":["Devastating Onslaught"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Devastating Onslaught","mana":"CCCCCRR","targets":["p0:Grizzly Bears"],"answers":[{"kind":"choose","pick":["X = 2"]}]}` + passes(1) + `,{"op":"attack","seat":0,"attackers":[],"defender":"p1"}` + passes(5) + `]}`
	peak, left := atEOTStackPeak(t, scenario, "Devastating Onslaught", "Grizzly Bears")
	if peak != 1 {
		t.Fatalf("%d Onslaught triggers on the stack at once, want 1", peak)
	}
	if left != 1 {
		t.Fatalf("%d Grizzly Bears left, want 1 (the original)", left)
	}
}
