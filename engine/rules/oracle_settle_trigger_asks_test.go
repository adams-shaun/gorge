package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// lb-xpending: a trigger-order ask (CR 603.3b) or a "you may pay" triggered
// cost (CR 603.5) still pending when a step ends is answered BEFORE that
// step's checkpoint, as XMage does inside the pass that put the trigger on
// the stack.

// Twinflame Travelers doubles Healer of the Glade's ETB: two identical
// triggers, so the order ask is pending the moment the spell resolves.
const settleOrderScenario = `{
  "name": "settle-trigger-order",
  "setup": {"p0": {"battlefield": ["Twinflame Travelers"], "hand": ["Healer of the Glade"]}, "p1": {"battlefield": ["Grizzly Bears"]}},
  "steps": [
    {"op": "cast", "seat": 0, "card": "p0:Healer of the Glade", "mana": "G"},
    {"op": "pass", "seat": 0},
    {"op": "pass", "seat": 1},
    {"op": "pass_to", "seat": 0, "decision": "priority"}
  ]
}`

// Trystan, Penitent Culler's first-main trigger asks "pay {G}" while it
// resolves, scripted on the resolve step: the pay election comes first and
// its plain pay taps the Forest through the planner (respay.go).
const settleCostScenario = `{
  "name": "settle-trigger-cost",
  "setup": {"p0": {"battlefield": ["Trystan, Callous Cultivator", "Forest"], "back_face": ["Trystan, Callous Cultivator"]}, "p1": {"battlefield": ["Grizzly Bears"]}},
  "steps": [
    {"op": "pass_to", "seat": 0, "decision": "priority"},
    {"op": "pass", "seat": 0},
    {"op": "pass", "seat": 1},
    {"op": "resolve", "seat": 0, "answers": [{"kind": "choose", "pick": ["trigger_cost_pay"]}]}
  ]
}`

func runSettleScenario(t *testing.T, scenario string) *oracleRun {
	t.Helper()
	sc, err := decodeOracleScenario([]byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	return run
}

func TestSettleTriggerOrderBeforeCheckpoint(t *testing.T) {
	run := runSettleScenario(t, settleOrderScenario)
	// snaps: setup, step 0 cast, 1 pass, 2 pass, 3 pass_to.
	snap := run.snaps[3]
	if !strings.Contains(snap.Checkpoint, "step 2") {
		t.Fatalf("snaps[3] is %q, want step 2", snap.Checkpoint)
	}
	if got := len(snap.Stack); got != 2 {
		t.Fatalf("step 2 stack has %d items, want the two ordered Healer triggers", got)
	}
	found := false
	for _, d := range run.decisions {
		if d.GorgeKind == "trigger_order" {
			found = true
			if d.Step != 2 {
				t.Fatalf("trigger_order recorded at step %d, want 2 (the pass that posed it)", d.Step)
			}
		}
	}
	if !found {
		t.Fatalf("no trigger_order decision recorded: the scenario no longer exercises the ask\n%s", strings.Join(run.log, "\n"))
	}
}

func TestSettleTriggerCostBeforeCheckpoint(t *testing.T) {
	run := runSettleScenario(t, settleCostScenario)
	var snap *OracleSnapshot
	for i := range run.snaps {
		if strings.Contains(run.snaps[i].Checkpoint, "step 2") {
			snap = &run.snaps[i]
		}
	}
	if snap == nil {
		t.Fatal("no step 2 snapshot")
	}
	tapped := false
	for _, p := range snap.Permanents {
		if p.Name == "Forest" {
			tapped = p.Tapped
		}
	}
	if !tapped {
		t.Fatalf("Forest untapped at step 2: the {G} trigger cost was still pending at the checkpoint\n%s", strings.Join(run.log, "\n"))
	}
	for _, d := range run.decisions {
		if len(d.PickKinds) > 0 && d.PickKinds[0] == "trigger_cost_pay" && d.Step != 2 {
			t.Fatalf("trigger cost answered at step %d, want 2", d.Step)
		}
	}
}
