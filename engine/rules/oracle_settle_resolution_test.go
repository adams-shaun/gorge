package rules

import (
	"strings"
	"testing"
)

// lb4w2-diverge: a `pass` that completes a priority round resolves the top
// object (CR 117.4, 608.1); when that resolution asks (Occult Epiphany's
// "then discard X cards", CR 608.2c) the generated scenario's checkpoint
// shows the finished resolution, as XMage's pass pair does.
const settleResolutionScenario = `{
  "name": "settle-resolution-ask",
  "setup": {"p0": {"battlefield": ["Entropic Battlecruiser"], "counters": {"Entropic Battlecruiser": {"CHARGE": 1}}},
            "p1": {"battlefield": ["Grizzly Bears"], "hand": ["Occult Epiphany", "Grizzly Bears"]}},
  "steps": [
    {"op": "pass", "seat": 0},
    {"op": "cast", "seat": 1, "card": "p1:Occult Epiphany", "mana": "CCU", "answers": [{"kind": "choose", "pick": ["X = 2"]}]},
    {"op": "pass", "seat": 1},
    {"op": "pass", "seat": 0},
    {"op": "pass_to", "seat": 0, "decision": "priority"}
  ]
}`

func TestSettleResolutionAskBeforeCheckpoint(t *testing.T) {
	run := runSettleScenario(t, settleResolutionScenario)
	// snaps: setup, step 0 pass, 1 cast, 2 pass, 3 pass, 4 pass_to.
	snap := run.snaps[4]
	if !strings.Contains(snap.Checkpoint, "step 3") {
		t.Fatalf("snaps[4] is %q, want step 3", snap.Checkpoint)
	}
	p1 := snap.Players[1]
	if len(p1.Hand) != 1 {
		t.Fatalf("step 3 p1.hand = %v, want the one card left after drawing two and discarding two", p1.Hand)
	}
	if got := len(snap.Stack); got != 2 {
		t.Fatalf("step 3 stack has %d items, want Entropic Battlecruiser's two discard triggers", got)
	}
	for _, d := range run.decisions {
		if d.GorgeKind == "modes" && d.Step != 3 {
			t.Fatalf("discard recorded at step %d, want 3 (the pass that resolved Occult Epiphany)", d.Step)
		}
	}
}
