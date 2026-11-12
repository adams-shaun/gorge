package rules

// The batch-amount shape of Mode$ CounterAdded (the row's "CounterAdded models
// only the crossing gate, not the batch amount").
//
// trigmatch.counterAddedMatches already fires on the CounterChange put and already reads
// the event's whole placement batch for its crossing gate (CounterAmount$).
// What a plain CounterAdded trigger had no way to read was the batch itself:
// triggerReferents bound TriggerAmount/TriggerCard for CounterAddedOnce but
// not for CounterAdded, so a body written "put/draw that many" resolved
// TriggerCount$Amount to 0. Both modes share the one matcher, so both must
// bind the one batch role.
//
// TestCounterAddedBatchAmountReferentOnRealCard pins the binding on the real
// corpus CounterAdded card Bloodcrazed Hoplite (the row's "whenever a +1/+1
// counter is put on CARDNAME" shape). TestCounterAddedBatchAmountIsReadByTheBody
// drives the same mode through the trigger queue and a body that reads
// TriggerCount$Amount. A plain line fires once per counter
// (CR 603.2c), so the batch now queues one instance per counter and the payoff
// driver lives in counteradded_per_counter_test.go; this test pins the
// queue count per event instead.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestCounterAddedBatchAmountReferentOnRealCard pins that a plain CounterAdded
// trigger binds the batch amount and the counter recipient from its
// CounterChange event. Bloodcrazed Hoplite is a real Mode$ CounterAdded |
// ValidCard$ Card.Self | CounterType$ P1P1 line, so this is the shipped card
// shape and not a synthetic one.
func TestCounterAddedBatchAmountReferentOnRealCard(t *testing.T) {
	t.Parallel()
	hoplite := corpusAlternativeCard(t, "Bloodcrazed Hoplite")
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, hoplite)

	// Precondition: locate the card's Mode$ CounterAdded line. The card's
	// trigger[0] is a keyword-granted spell-cast trigger, so a fixed index
	// would pin the wrong line and the referent would be bound by an
	// unrelated case.
	var trig cards.Trigger
	found := false
	for _, tr := range e.G.Obj(id).Face().Triggers {
		if tr.Mode == "CounterAdded" {
			trig, found = tr, true
			break
		}
	}
	if !found {
		t.Fatalf("Bloodcrazed Hoplite carries no Mode$ CounterAdded trigger")
	}

	ctx := e.triggerReferents(trig, id, events.Event{
		Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 3,
	}, nil)
	// Precondition: the two values the binding must produce differ from zero,
	// so a pass cannot be the incidental zero.
	if ctx.TriggerAmount != 3 {
		t.Fatalf("TriggerAmount = %d, want 3 (the CounterChange batch size)", ctx.TriggerAmount)
	}
	if ctx.TriggerCard != id {
		t.Fatalf("TriggerCard = %d, want %d (the counter recipient)", ctx.TriggerCard, id)
	}
}

// TestCounterAddedBatchAmountIsReadByTheBody drives Mode$ CounterAdded end to
// end. A PLAIN "whenever a counter is put" line fires once
// per counter (CR 603.2c), so a 3-counter batch queues three instances and the
// payoff is three draws -- the same total the batch-of-one account produced
// when one instance read TriggerCount$Amount = 3. What this test still pins is
// that the payoff SCALES with the event rather than being hard-coded: the
// second, 2-counter batch draws two more. A batch of a different counter kind
// queues nothing (CounterType$ P1P1).
//
// The card text is synthesized because no corpus Mode$ CounterAdded card
// spells a "that many" body -- the corpus spells it on the Once/All variants,
// which the row holds separately -- but the trigger line is exactly the real
// CounterAdded grammar these tests exercise.
func TestCounterAddedBatchAmountIsReadByTheBody(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	src := "Name:Batch Chronicler\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n" +
		"T:Mode$ CounterAdded | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ P1P1 | Execute$ TrigDraw\n" +
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ X\n" +
		"SVar:X:TriggerCount$Amount\n"
	id := onBoard(t, e, 0, src)

	trig := e.G.Obj(id).Face().Triggers[0]
	if trig.Mode != "CounterAdded" || trig.Params["Execute"] != "TrigDraw" {
		t.Fatalf("synthetic trigger = %q/%q, want CounterAdded/TrigDraw", trig.Mode, trig.Params["Execute"])
	}

	// One batch of three queues ONE INSTANCE PER COUNTER (CR 603.2c);
	// that is the count gate here. Driving them to a payoff
	// is counteradded_per_counter_test.go's job -- a fixture that drains
	// them by hand must first answer the CR 603.3b order question they pose.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 3})
	if len(e.pendingTriggers) != 3 {
		t.Fatalf("pendingTriggers after a 3-counter batch = %d, want 3", len(e.pendingTriggers))
	}
	e.pendingTriggers = nil

	// A different batch size (2) queues two more, not a repeat of the
	// first batch's three.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	if len(e.pendingTriggers) != 2 {
		t.Fatalf("pendingTriggers after a second 2-counter batch = %d, want 2", len(e.pendingTriggers))
	}
	e.pendingTriggers = nil

	// The counter kind still gates: a batch of a different kind is not
	// "a +1/+1 counter" and queues nothing.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 4})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("pendingTriggers after a CHARGE batch = %d, want 0", len(e.pendingTriggers))
	}
}
