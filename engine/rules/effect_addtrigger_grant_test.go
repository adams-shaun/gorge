package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectAddTriggerGrantRemembered pins an Effect-delivered Mode$
// Continuous static whose only grant is AddTrigger$ over Card.IsRemembered
// (Olinda the Oblivious, Obsidian Fireheart), on the real corpus card: the
// odor-countered creature gains "At the beginning of your upkeep, you lose 2
// life" -- its CONTROLLER loses the life, even after Olinda has left the
// battlefield -- and the grant ends with the last odor counter
// (ForgetCounter$ ODOR).
func TestEffectAddTriggerGrantRemembered(t *testing.T) {
	t.Parallel()
	e, cfg := phaseRecurEngine(t, []string{"Olinda the Oblivious"}, []string{"Grizzly Bears"})
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	olinda := moveByName(t, e, 0, "Olinda the Oblivious", state.ZHand)
	addMana(t, e, 0, "BBBB")
	castByID(t, e, olinda)
	// The ETB trigger's target ask (up to one creature the opponent controls
	// without an odor counter) and the resolution chain.
	for i := 0; i < 40 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving Olinda")
		}
		if d.Kind == decision.KPriority {
			submitPass(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	// Preconditions: the odor counter landed, the effect registered a trigger
	// grant remembering exactly that creature, and its controller is seat 1.
	if got := e.G.Obj(bear).Counter("ODOR"); got != 1 {
		t.Fatalf("precondition: the opposing bear has %d odor counters, want 1", got)
	}
	var grant *ContinuousEffect
	for i := range e.continuous {
		if ce := &e.continuous[i]; ce.AddTrigger != nil && ce.ForgetCounter == "ODOR" {
			grant = ce
		}
	}
	if grant == nil {
		t.Fatal("no registered AddTrigger grant with ForgetCounter ODOR")
	}
	if len(grant.Remembered) != 1 || grant.Remembered[0] != bear {
		t.Fatalf("grant Remembered = %v, want [%d]", grant.Remembered, bear)
	}
	// "They continue to smell after Olinda has left the battlefield."
	e.emit(events.Event{Kind: events.MoveZone, Obj: olinda, From: state.ZBattlefield, To: state.ZGraveyard})

	life := e.G.Players[1].Life
	passUntilStep(t, e, 2, state.StepUpkeep)
	passUntilStep(t, e, 2, state.StepDraw)
	if got := e.G.Players[1].Life; got != life-2 {
		t.Fatalf("seat 1 life after its upkeep = %d, want %d (the odor creature's trigger)", got, life-2)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20 (the trigger is the creature's controller's)", got)
	}

	// Removing the last odor counter ends the grant: no loss at the next
	// seat-1 upkeep.
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "ODOR", Amount: -1})
	if got := e.G.Obj(bear).Counter("ODOR"); got != 0 {
		t.Fatalf("precondition: odor counters after removal = %d, want 0", got)
	}
	life = e.G.Players[1].Life
	passUntilStep(t, e, 4, state.StepDraw)
	if got := e.G.Players[1].Life; got != life {
		t.Fatalf("seat 1 life after the counter was removed = %d, want %d (no loss)", got, life)
	}
	replayCheck(t, e, cfg)
}
