package rules

// A delayed trigger created by a trigger whose registration has no
// RememberObjects$ remembers nothing in Forge: its body's Defined$ Remembered is
// exactly what the body itself remembered (state.DelayedTrigger.CaptureOnly).
// Fortune, Loyal Steed's end-of-combat flicker is the measured case: the attack
// trigger's capture (every attacker plus the defending player) must not be
// returned along with the cards the body exiled.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// fortuneScene is Fortune (summoning sickness cleared, ETB scry drained), a
// Grizzly Bears and two Runeclaw Bears on seat 0's battlefield; b1 saddled
// Fortune.
func fortuneScene(t *testing.T) (e *Engine, fortune, grizzly, b1 state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	extra := append([]*cards.Card{lookup(t, reg, "Fortune, Loyal Steed"), lookup(t, reg, "Grizzly Bears")},
		saddleExtraCards(t, reg)...)
	e = corpusEngine(t, reg, extra, nil)
	fortune = moveByName(t, e, 0, "Fortune, Loyal Steed", state.ZBattlefield)
	grizzly = moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	b1, _ = saddleBears(t, e)
	e.G.Obj(fortune).SummonSick = false
	e.G.Obj(grizzly).SummonSick = false
	// Fortune's ETB scry is pending; put it on the stack and resolve it so the
	// Saddle ability is offered.
	e.pending = nil
	e.priorityRound()
	fortuneDrain(t, e, 0)
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: stack not empty after the scry drain: %d", len(e.G.Stack))
	}
	d8PayTapElection(t, e, fortune, 1, b1)
	return
}

// fortuneDrain passes priority until the stack is empty, answering a scry
// arrangement with its minimum and an end-of-combat "Choose card" with pick.
func fortuneDrain(t *testing.T, e *Engine, pick state.ObjID) {
	t.Helper()
	for i := 0; i < 60 && (len(e.G.Stack) > 0 || (e.Pending() != nil && e.Pending().Kind != decision.KPriority)); i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while draining")
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KChoose:
			idx := []int{}
			for _, o := range d.Options {
				if pick != 0 && o.Obj == pick {
					idx = []int{o.Index}
				}
			}
			if pick != 0 && len(idx) == 0 {
				t.Fatalf("precondition: the saddler %d is not offered: %+v", pick, d.Options)
			}
			submitChoices(t, e, idx...)
		default:
			fdnCensusAnswerMin(t, e, d)
		}
	}
}

// fortuneAttackWith declares every id as an attacker and resolves the attack
// trigger.
func fortuneAttackWith(t *testing.T, e *Engine, ids ...state.ObjID) {
	t.Helper()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("attacker declaration = %+v, want KAttackers", d)
	}
	var choices []int
	for _, id := range ids {
		for _, o := range d.Options {
			if o.Obj == id {
				choices = append(choices, o.Index)
			}
		}
	}
	if len(choices) != len(ids) {
		t.Fatalf("precondition: %d of %d attackers offered: %+v", len(choices), len(ids), d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("declare attackers: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("precondition: %d delayed triggers registered, want 1", len(e.G.Delayed))
	}
	if !e.G.Delayed[0].CaptureOnly {
		t.Fatal("precondition: Fortune's registration is not capture-only")
	}
}

// fortuneToMain2 drives to the second main phase, choosing pick at the
// end-of-combat election.
func fortuneToMain2(t *testing.T, e *Engine, pick state.ObjID) {
	t.Helper()
	for i := 0; i < 400 && !(e.G.Step == state.StepMain2 && e.G.Active == 0); i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while driving to Main2")
		}
		switch d.Kind {
		case decision.KPriority:
			submitPass(t, e)
		case decision.KChoose:
			fortuneDrainChoose(t, e, d, pick)
		case decision.KBlockers:
			submitChoices(t, e)
		default:
			fdnCensusAnswerMin(t, e, d)
		}
	}
	fortuneDrain(t, e, pick)
	if e.G.Step != state.StepMain2 {
		t.Fatalf("precondition: stopped at %s, want Main2", e.G.Step)
	}
}

func fortuneDrainChoose(t *testing.T, e *Engine, d *decision.Decision, pick state.ObjID) {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == pick {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("precondition: %d is not offered: %+v", pick, d.Options)
}

// fortuneMovesOf counts MoveZone events for obj from..to at or after log index from.
func fortuneMovesOf(e *Engine, since int, obj state.ObjID, from, to state.Zone) int {
	n := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.MoveZone && ev.Obj == obj && ev.From == from && ev.To == to {
			n++
		}
	}
	return n
}

func fortuneAnyMoveOf(e *Engine, since int, obj state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.MoveZone && ev.Obj == obj {
			n++
		}
	}
	return n
}

func TestFortuneEndCombatFlickerReturnsOnlyExiled(t *testing.T) {
	e, fortune, grizzly, b1 := fortuneScene(t)
	since := len(e.L.Events)
	fortuneAttackWith(t, e, fortune, grizzly)
	if e.G.Obj(grizzly).Zone != state.ZBattlefield || e.G.Obj(fortune).Zone != state.ZBattlefield {
		t.Fatal("precondition: attackers are not on the battlefield")
	}
	afterAttack := len(e.L.Events)
	fortuneToMain2(t, e, b1)
	for _, id := range []state.ObjID{fortune, b1, grizzly} {
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Errorf("object %d in zone %v, want the battlefield", id, z)
		}
	}
	if n := fortuneMovesOf(e, since, fortune, state.ZExile, state.ZBattlefield); n != 1 {
		t.Errorf("Fortune returned %d times, want 1", n)
	}
	if n := fortuneMovesOf(e, since, b1, state.ZExile, state.ZBattlefield); n != 1 {
		t.Errorf("the saddler returned %d times, want 1", n)
	}
	if n := fortuneAnyMoveOf(e, since, grizzly); n != 0 {
		t.Errorf("Grizzly Bears moved %d times, want 0", n)
	}
	for _, id := range []state.ObjID{fortune, b1} {
		if n := fortuneMovesOf(e, since, id, state.ZBattlefield, state.ZBattlefield); n != 0 {
			t.Errorf("object %d moved battlefield->battlefield %d times, want 0", id, n)
		}
	}
	// One return of Fortune means one ETB scry trigger after end of combat.
	scry := 0
	for _, ev := range e.L.Events[afterAttack:] {
		if ev.Kind == events.TriggerPush && ev.Obj == fortune {
			scry++
		}
	}
	if scry != 1 {
		t.Errorf("Fortune's ETB scry triggered %d times after the attack trigger, want 1", scry)
	}
}

func TestFortuneDiedInCombatStaysDead(t *testing.T) {
	e, fortune, grizzly, b1 := fortuneScene(t)
	since := len(e.L.Events)
	fortuneAttackWith(t, e, fortune, grizzly)
	e.emit(events.Event{Kind: events.MoveZone, Obj: fortune, From: state.ZBattlefield, To: state.ZGraveyard})
	if z := e.G.Obj(fortune).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: Fortune is in zone %v, want the graveyard", z)
	}
	fortuneToMain2(t, e, b1)
	if z := e.G.Obj(fortune).Zone; z != state.ZGraveyard {
		t.Errorf("dead Fortune is in zone %v, want the graveyard", z)
	}
	if n := fortuneAnyMoveOf(e, since, grizzly); n != 0 {
		t.Errorf("Grizzly Bears moved %d times, want 0", n)
	}
	if n := fortuneMovesOf(e, since, fortune, state.ZGraveyard, state.ZBattlefield); n != 0 {
		t.Errorf("Fortune left the graveyard %d times, want 0", n)
	}
}
