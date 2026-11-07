package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAnimateGrantedDamageTriggerKeepsOtherObserversTriggers pins the damage
// checkpoint where an Animate-granted Mode$ DamageDone line (Dreadmaw's Ire's
// DamageTrig on Hill Giant) and the damaged player's own DamageDoneOnce line
// (Contested Game Ball) both watch the same combat Damage event. CR 603.2 and
// CR 603.3b: both abilities trigger, and the batch goes on the stack in APNAP
// order. The RunOracleScenarioJSON checkpoint stands in for Forge/XMage, which
// settle the granted line's CR 603.3d target ask inside the pass: with that ask
// left pending, the Ball's trigger was still queued behind it and the
// checkpoint showed only the attacker's trigger.
func TestAnimateGrantedDamageTriggerKeepsOtherObserversTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	raw := `{"name": "ire_ball", "turn": 1, "cr": [], "why": "CR 603.2: an Animate-granted DamageDone line and the damaged player's own DamageDoneOnce line both trigger on one combat Damage event", "setup": {"p0": {"battlefield": ["Mountain", "Hill Giant"], "hand": ["Dreadmaw's Ire"], "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20}, "p1": {"battlefield": ["Contested Game Ball", "Ornithopter"], "hand": [], "library_top": ["Island", "Island", "Island", "Island", "Island"], "life": 20}}, "steps": [{"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "attack", "seat": 0, "attackers": ["p0:Hill Giant"], "defender": "p1"}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "block", "seat": 1}, {"op": "activate", "seat": 0, "card": "p0:Mountain", "ability_index": 0}, {"op": "cast", "seat": 0, "card": "p0:Dreadmaw's Ire", "targets": ["p0:Hill Giant"]}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0, "answers": [{"kind": "target", "pick": ["p1:Ornithopter"]}]}, {"op": "pass", "seat": 1}, {"op": "pass", "seat": 0}, {"op": "pass", "seat": 1}], "expect": []}`
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, _, run := runOracleScenarioSpare(reg, sc, false, new(Spare))
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v", fails)
	}
	if len(run.snaps) == 0 {
		t.Fatal("precondition: the scenario took no checkpoint")
	}
	// Find the checkpoint taken right after the combat Damage event. The
	// snapshot lists the stack top first: CR 603.3b APNAP puts the active
	// player's Ire-granted trigger on the stack first (bottom), then the
	// defending player's Ball trigger (top) -- the order Forge and XMage
	// report. Without both triggers the Ball's line is still queued behind
	// the granted line's unanswered target ask.
	damage := -1
	for i := range run.snaps {
		sn := &run.snaps[i]
		if len(sn.Stack) == 2 &&
			sn.Stack[0].Source == "p1:Contested Game Ball" && sn.Stack[1].Source == "p0:Hill Giant" {
			damage = i
			break
		}
	}
	if damage < 0 {
		t.Fatal("no checkpoint carries both triggers of the damage batch")
	}
	cp := run.snaps[damage]
	// Precondition: the combat damage really happened before this checkpoint,
	// so a two-trigger stack cannot be an unrelated earlier state.
	if cp.Players[1].Life >= 20 {
		t.Fatalf("precondition: p1 life = %d at the checkpoint, want combat damage dealt", cp.Players[1].Life)
	}
	if cp.Stack[0].Kind != "ability" || cp.Stack[1].Kind != "ability" {
		t.Fatalf("damage checkpoint stack kinds = %q, %q, want two abilities", cp.Stack[0].Kind, cp.Stack[1].Kind)
	}
	// The trigger really fires: the Ball's ability is the first to resolve,
	// so its controller changes to the attacking player. The granted line's
	// destroy took the scripted Ornithopter, leaving the Ball alive.
	ball := run.e.G.Obj(run.refs["p1:Contested Game Ball"])
	if ball == nil {
		t.Fatal("precondition: the Ball is on the battlefield")
	}
	if ball.Controller != 0 {
		t.Fatalf("Ball controller = %d, want 0 (the attacking player): the Ball's trigger never resolved", ball.Controller)
	}
	replayCheck(t, run.e, run.cfg)
}
