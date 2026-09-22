package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

const gameLimitedPhaseTrigger = "Name:OncePhase\nManaCost:0\nTypes:Creature\nPT:2/2\n" +
	"T:Mode$ Phase | Phase$ Main2 | ValidPlayer$ You | TriggerZones$ Battlefield | GameActivationLimit$ 1 | Execute$ TrigDraw\n" +
	"SVar:TrigDraw:DB$ Draw | NumCards$ 1\nOracle:x\n"

func TestTriggerGameActivationLimitPhaseSurvivesTurnAndClone(t *testing.T) {
	e, _, id := newFixtureDeck(t, 71, gameLimitedPhaseTrigger)
	moveByName(t, e, 0, "OncePhase", state.ZBattlefield)
	tr := crTriggerFixture(t, e, id, "Phase", "Draw")
	if tr.Params["GameActivationLimit"] != "1" || tr.Params["Phase"] != "Main2" {
		t.Fatalf("fixture trigger parameters changed: %+v", tr.Params)
	}
	if e.G.Obj(id).Zone != state.ZBattlefield || e.G.Active != 0 {
		t.Fatalf("precondition: source/active player = %+v/%d", e.G.Obj(id), e.G.Active)
	}

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if got := queuedPhaseTriggers(e, id); got != 1 {
		t.Fatalf("first eligible phase queued %d triggers, want 1", got)
	}
	e.pendingTriggers = nil
	clone := e.Clone()
	cloneKey := triggerKey{Source: id, Idx: 0, Face: 0}
	if clone.triggerGameFires == nil || clone.triggerGameFires[cloneKey] != 1 {
		t.Fatalf("clone did not preserve game trigger count: %+v", clone.triggerGameFires)
	}
	clone.triggerGameFires[cloneKey]++
	if e.triggerGameFires[cloneKey] != 1 {
		t.Fatal("clone shares game trigger count map with original")
	}

	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	if e.G.Turn != 2 || e.G.Active != 0 {
		t.Fatalf("precondition: turn/active after TurnChange = %d/%d", e.G.Turn, e.G.Active)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if got := queuedPhaseTriggers(e, id); got != 0 {
		t.Fatalf("game-limited trigger re-fired on a later turn: %d", got)
	}

	clone.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	clone.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if got := queuedPhaseTriggers(clone, id); got != 0 {
		t.Fatalf("cloned engine re-armed game-limited trigger: %d", got)
	}
}

func TestTriggerGameActivationLimitRealCorpusAcrobaticCheerleader(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Acrobatic Cheerleader")
	if !ok {
		t.Fatal("corpus fixture: Acrobatic Cheerleader missing")
	}
	e := New(seatZeroStart(Config{Seed: 72, Names: []string{"a", "b"}, Decks: [][]*cards.Card{
		append(mountainDeck(t, 40), card), mountainDeck(t, 41),
	}}))
	e.Advance()
	id := crAbortMove(t, e, 0, "Acrobatic Cheerleader", state.ZBattlefield)
	e.pending = nil
	tr := crTriggerFixture(t, e, id, "Phase", "PutCounter")
	if tr.Params["GameActivationLimit"] != "1" || tr.Params["Phase"] != "Main" || tr.Params["PhaseCount"] != "2" || tr.Params["ValidPlayer"] != "You" {
		t.Fatalf("Acrobatic Cheerleader trigger parameters changed: %+v", tr.Params)
	}
	if e.G.Obj(id).Zone != state.ZBattlefield || e.G.Active != 0 {
		t.Fatalf("precondition: source/active player = %+v/%d", e.G.Obj(id), e.G.Active)
	}
	e.emit(events.Event{Kind: events.Tap, Obj: id})
	if !e.G.Obj(id).Tapped {
		t.Fatal("precondition: source was not tapped")
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if got := queuedPhaseTriggers(e, id); got != 1 {
		t.Fatalf("Acrobatic Cheerleader did not queue on eligible second main: %d", got)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain2})
	if got := queuedPhaseTriggers(e, id); got != 0 {
		t.Fatalf("Acrobatic Cheerleader queued again after its game limit: %d", got)
	}
}
