package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestCountersAddedThisTurn(t *testing.T) {
	e := layerEngine(t)
	tarfire := onBoardCard(t, e, 0, yourCountersCard(t, "Lasting Tarfire"))
	creature := onBoardCard(t, e, 0, yourCountersCard(t, "Wakka, Devoted Guardian"))
	other := onBoardCard(t, e, 1, yourCountersCard(t, "Wakka, Devoted Guardian"))
	if e.G.Obj(tarfire).Zone != state.ZBattlefield || e.G.Obj(creature).Zone != state.ZBattlefield {
		t.Fatal("test precondition: sources are not on the battlefield")
	}
	body := svarBodyOf(t, e.G.Obj(tarfire).Face(), "X")
	if body != "Count$CountersAddedThisTurn Any You Creature" {
		t.Fatalf("test precondition: Lasting Tarfire X = %q", body)
	}
	ctx := &effects.Ctx{Controller: 0, Source: tarfire}
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 0 {
		t.Fatalf("baseline = %d (ok %v), want evaluated zero", n, ok)
	}

	// Publish the real adder role, as cost/turn-based placement sites do.
	prev := e.SetCounterAdder(0)
	e.emit(events.Event{Kind: events.CounterChange, Obj: creature, Counter: "P1P1", Amount: 2})
	e.SetCounterAdder(prev)
	if n, ok := effects.EvalCountOK(e, ctx, body); !ok || n != 2 {
		t.Fatalf("matching placement = %d (ok %v), want 2", n, ok)
	}
	// A placement by seat 1 and a non-creature placement are isolated.
	prev = e.SetCounterAdder(1)
	e.emit(events.Event{Kind: events.CounterChange, Obj: other, Counter: "P1P1", Amount: 7})
	e.SetCounterAdder(prev)
	prev = e.SetCounterAdder(0)
	e.emit(events.Event{Kind: events.CounterChange, Obj: tarfire, Counter: "LORE", Amount: 3})
	e.SetCounterAdder(prev)
	if n, _ := effects.EvalCountOK(e, ctx, body); n != 2 {
		t.Fatalf("isolated placements changed You Creature count to %d", n)
	}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CountersAddedThisTurn LORE You Card.Self"); !ok || n != 3 {
		t.Fatalf("Card.Self LORE count = %d (ok %v), want 3", n, ok)
	}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CountersAddedThisTurn P1P1 Player Permanent.YouCtrl"); !ok || n != 2 {
		t.Fatalf("Player Permanent.YouCtrl count = %d (ok %v), want 2", n, ok)
	}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CountersAddedThisTurn Any You Creature"); !ok || n != 2 {
		t.Fatalf("Any count = %d (ok %v), want 2", n, ok)
	}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CountersAddedThisTurn P1P1 You Creature"); !ok || n != 2 {
		t.Fatalf("P1P1 matching creature count = %d (ok %v), want 2", n, ok)
	}

	clone := e.Clone()
	if n, _ := effects.EvalCountOK(clone, ctx, body); n != 2 {
		t.Fatalf("clone count = %d, want 2", n)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	if n, _ := effects.EvalCountOK(e, ctx, body); n != 0 {
		t.Fatalf("reset count = %d, want 0", n)
	}
	if n, _ := effects.EvalCountOK(clone, ctx, body); n != 2 {
		t.Fatalf("clone changed when original reset: %d", n)
	}
}

func TestLastingTarfire(t *testing.T) {
	e := layerEngine(t)
	tarfire := onBoardCard(t, e, 0, yourCountersCard(t, "Lasting Tarfire"))
	creature := onBoardCard(t, e, 0, yourCountersCard(t, "Wakka, Devoted Guardian"))
	face := e.G.Obj(tarfire).Face()
	trig := face.Triggers[0]
	if trig.Mode != "Phase" || trig.Params["Phase"] != "End of Turn" || trig.Params["CheckSVar"] != "X" {
		t.Fatalf("test precondition: Lasting Tarfire trigger = %+v", trig)
	}
	prev := e.SetCounterAdder(0)
	e.emit(events.Event{Kind: events.CounterChange, Obj: creature, Counter: "P1P1", Amount: 1})
	e.SetCounterAdder(prev)
	if n, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0, Source: tarfire}, "Count$CountersAddedThisTurn Any You Creature"); !ok || n != 1 {
		t.Fatalf("matching trigger count = %d (ok %v), want 1", n, ok)
	}
	for i := range e.G.Players {
		e.G.Players[i].Life = 20
	}
	e.pending = nil
	e.setStep(state.StepEnd)
	e.priorityRound()
	if e.Pending() != nil {
		choices := make([]int, len(e.Pending().Options))
		for i, opt := range e.Pending().Options {
			choices[i] = opt.Index
		}
		submitChoices(t, e, choices...)
	}
	passUntilStackEmpty(t, e, 80)
	if e.G.Players[1].Life != 18 {
		t.Fatalf("Lasting Tarfire opponent life = %d, want 18", e.G.Players[1].Life)
	}

	// Control: the same compiled trigger with no matching placement must not
	// manufacture a damage trigger.
	e2 := layerEngine(t)
	onBoardCard(t, e2, 0, yourCountersCard(t, "Lasting Tarfire"))
	e2.pending = nil
	e2.setStep(state.StepEnd)
	e2.priorityRound()
	if e2.G.Players[1].Life != 20 {
		t.Fatalf("nonmatching Lasting Tarfire control life = %d, want 20", e2.G.Players[1].Life)
	}
}

func TestCountersAddedThisTurnMalformedIsUnresolvable(t *testing.T) {
	e := layerEngine(t)
	c, ok := testutil.CorpusRegistry(t).Lookup("Lasting Tarfire")
	if !ok {
		t.Fatal("corpus missing Lasting Tarfire")
	}
	id := onBoardCard(t, e, 0, c)
	ctx := &effects.Ctx{Controller: 0, Source: id}
	if n, ok := effects.EvalCountOK(e, ctx, "Count$CountersAddedThisTurn Any You"); ok || n != 0 {
		t.Fatalf("malformed count = %d (ok %v), want unresolvable", n, ok)
	}
}
