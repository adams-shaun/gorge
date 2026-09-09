package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestSuspendedResolutionLogsPriorityOnlyAtCompletion pins the priority-log
// contract this task restores: CR 117.5, nobody receives priority in the
// middle of a resolution. When a resolution suspends on a mid-resolution ask
// (a modal spell's KModes, an unless-pay, a discard), the engine is parked on
// that question and NOTHING has priority — so the log must carry no Priority
// event between the mid-resolution DecisionAsk and the answer. Before the
// fix (rules/legal.go) the pass-branch's pass-count reset emit ran
// unconditionally and logged Priority{active} right after that DecisionAsk,
// while the resolution was still suspended: a log lie, and the source of the
// host boundsOf mis-derivation.
//
// The companion property: once the resolution completes, the pass count
// resets and the "back to active" marker lands AFTER the object leaves the
// stack (the completion grant in rules/resolution.go, plus the normal
// grantPriority tail), never during the suspension. The engine's established
// shape for a completed resolution is the reset marker immediately followed by
// the round grant, so the leaf asserts the reset-with-zero holds after the
// object's completion move rather than counting Priority events (the two-event
// convention is the same for an unsuspended resolution, pinned elsewhere).
func TestSuspendedResolutionLogsPriorityOnlyAtCompletion(t *testing.T) {
	charm := "Name:PiC\nManaCost:R\nTypes:Instant\nA:SP$ Charm | Choices$ DoGain,DoLose\n" +
		"SVar:DoGain:DB$ GainLife | Defined$ You | LifeAmount$ 5 | SpellDescription$ Gain 5 life\n" +
		"SVar:DoLose:DB$ LoseLife | Defined$ You | LifeAmount$ 5 | SpellDescription$ Lose 5 life\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 91, charm)
	addMana(t, e, 0, "R")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected a suspended KModes decision, got %+v", d)
	}
	if o := e.G.Obj(id); o.Zone != state.ZStack {
		t.Fatalf("the charm must suspend with the spell still on the stack, zone %s", o.Zone)
	}

	// Find the mid-resolution DecisionAsk's seq and the seq of the answer that
	// follows it in the log (the next DecisionMade).
	askIdx := -1
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		if e.L.Events[i].Kind == events.DecisionAsk && e.L.Events[i].Text == "modes" {
			askIdx = i
			break
		}
	}
	if askIdx < 0 {
		t.Fatal("no mid-resolution 'modes' DecisionAsk in the log")
	}
	askSeq := e.L.Events[askIdx].Seq

	// Answer the modes question and run the resolution out.
	submitChoices(t, e, 1)
	passUntilStackEmpty(t, e, 20)

	// Find the answer's DecisionMade seq (the first DecisionMade after the
	// ask) and assert no Priority sits between the ask and the answer.
	ansIdx := -1
	for i := askIdx + 1; i < len(e.L.Events); i++ {
		if e.L.Events[i].Kind == events.DecisionMade {
			ansIdx = i
			break
		}
	}
	if ansIdx < 0 {
		t.Fatal("no DecisionMade follows the mid-resolution ask")
	}
	// THE discriminating assertion: no Priority between the ask and its
	// answer. The old pass-branch emit put exactly one here.
	for i := askIdx + 1; i < ansIdx; i++ {
		if e.L.Events[i].Kind == events.Priority {
			t.Fatalf("Priority logged between the mid-resolution DecisionAsk (seq %d) and its answer (seq %d): seq %d player %d",
				askSeq, e.L.Events[ansIdx].Seq, e.L.Events[i].Seq, e.L.Events[i].Player)
		}
	}
	if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
		t.Fatalf("the charm resolved to %s, want Graveyard", z)
	}

	// The completion grant must land AFTER the object leaves the stack, with
	// the pass-count reset (Amount 0) that marks the round ending. Find the
	// charm's MoveZone off the stack, then the first Priority strictly after
	// it.
	moveIdx := -1
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZStack {
			moveIdx = i
			break
		}
	}
	if moveIdx < 0 {
		t.Fatal("the charm never left the stack after resolving")
	}
	found := false
	for i := moveIdx + 1; i < len(e.L.Events); i++ {
		if e.L.Events[i].Kind == events.Priority {
			if e.L.Events[i].Amount != 0 {
				t.Fatalf("priority grant after completion carries Amount %d, want 0 (pass count not reset)",
					e.L.Events[i].Amount)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no Priority (pass-count reset) logged after the suspended resolution completed")
	}

	replayCheck(t, e, cfg)
}
