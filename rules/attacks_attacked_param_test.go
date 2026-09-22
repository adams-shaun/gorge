package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Mode$ Attacks' Attacked$ scopes the trigger to the DECLARED DEFENDER
// (CR 508.1c): Revenge of Ravens' "Whenever a creature attacks you or a
// planeswalker you control" is Attacked$ You,Planeswalker.YouCtrl. Before
// attacksMatches read the param, the trigger fired on every DeclareAttackers
// event at the table -- right in a two-player game, over-broad in every
// multiplayer one.

// submitAttackerAt picks the KAttackers option naming id with defender def
// and submits exactly that one. Needed because a creature is offered once per
// living opponent (combat.go's defender-major option list), so the choice of
// DEFENDER is part of the intent.
func submitAttackerAt(t *testing.T, e *Engine, id state.ObjID, def state.PlayerID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == id && o.Player == def {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}); err != nil {
				t.Fatalf("submit attacker %d at %d: %v", id, def, err)
			}
			return
		}
	}
	t.Fatalf("no offered attacker option for obj %d at defender %d: %+v", id, def, d.Options)
}

// attacksTriggerSeat builds a three-seat table with Revenge of Ravens on seat
// 0 and one ready 0/4 attacker on seat 1, active on seat 1 in the
// declare-attackers step.
func attacksTriggerSeat(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := threeSeatEngine(t)
	ravens := mshCorpusCard(t, "Revenge of Ravens")
	onBoardCard(t, e, 0, ravens)
	// A 0-power attacker: it can attack (no minimum power) and deals no
	// combat damage, so the only life movement is the trigger's own drain and
	// gain. That keeps the assertions about the TRIGGER, not about combat.
	bear := onBoardReady(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
	e.G.Active = 1
	e.G.Step = state.StepDeclareAttackers
	return e, bear
}

// TestRevengeOfRavensFiresWhenItsControllerIsAttacked is the positive half:
// seat 1 attacks seat 0 (the trigger's controller) and the drain fires.
func TestRevengeOfRavensFiresWhenItsControllerIsAttacked(t *testing.T) {
	e, bear := attacksTriggerSeat(t)
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	e.askAttackers()
	submitAttackerAt(t, e, bear, 0)
	drainCombatPriority(t, e)

	if got := e.G.Players[0].Life; got != life0+1 {
		t.Fatalf("trigger controller life = %d, want %d (Revenge of Ravens gains 1)", got, life0+1)
	}
	if got := e.G.Players[1].Life; got != life1-1 {
		t.Fatalf("attacking controller life = %d, want %d (Revenge of Ravens drains 1)", got, life1-1)
	}
}

// TestRevengeOfRavensDoesNotFireWhenAThirdSeatIsAttacked is the defect: seat 1
// attacks seat 2, which is neither the trigger's controller nor a permanent
// that controller owns, so nothing drains and nothing is gained.
func TestRevengeOfRavensDoesNotFireWhenAThirdSeatIsAttacked(t *testing.T) {
	e, bear := attacksTriggerSeat(t)
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	e.askAttackers()
	submitAttackerAt(t, e, bear, 2)
	drainCombatPriority(t, e)

	if got := e.G.Players[0].Life; got != life0 {
		t.Fatalf("trigger controller life = %d, want %d (no trigger on a third seat)", got, life0)
	}
	if got := e.G.Players[1].Life; got != life1 {
		t.Fatalf("attacking controller life = %d, want %d (no drain on a third seat)", got, life1)
	}
}

// TestAttacksAttackedOpponentScopesToTheTriggersOpponent pins the opponent
// half of the shared player filter: Attacked$ Opponent matches a defender that
// is an opponent of the trigger's controller, and not the controller's own
// seat. Exercised on the engine matcher directly so the opponent-positive and
// controller-is-self-negative cases share one fixture.
func TestAttacksAttackedOpponentScopesToTheTriggersOpponent(t *testing.T) {
	e, _ := attacksTriggerSeat(t)
	// A source on seat 0: the perspective `you` is then seat 0, which makes
	// seat 2 the opponent under test.
	trig := onBoard(t, e, 0, "Name:Sentinel\nManaCost:1\nTypes:Artifact\nOracle:x\n")
	atk := onBoard(t, e, 1, "Name:Probe\nManaCost:0\nTypes:Artifact Creature Construct\nPT:0/1\nOracle:x\n")
	mk := func(attacked string) cards.Trigger {
		return cards.Trigger{Mode: "Attacks", Params: map[string]string{
			"ValidCard": "Creature", "Attacked": attacked,
		}}
	}
	evt := func(def state.PlayerID) events.Event {
		return events.Event{Kind: events.DeclareAttackers, Player: def, IDs: []state.ObjID{atk}}
	}

	// Attacked$ Opponent: seat 2 is an opponent of the source's controller
	// (seat 0), seat 0 is not.
	if !e.attacksMatches(mk("Opponent"), trig, evt(2)) {
		t.Fatal("Attacked$ Opponent should match a defender that is an opponent of the controller")
	}
	if e.attacksMatches(mk("Opponent"), trig, evt(0)) {
		t.Fatal("Attacked$ Opponent must not match the controller's own seat")
	}
	// Attacked$ You: the mirror image.
	if !e.attacksMatches(mk("You"), trig, evt(0)) {
		t.Fatal("Attacked$ You should match the controller's own seat")
	}
	if e.attacksMatches(mk("You"), trig, evt(2)) {
		t.Fatal("Attacked$ You must not match a third seat")
	}
}
