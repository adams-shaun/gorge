package rules

// CR 603.2c: an event with several occurrences of the triggering condition
// triggers that many times. A Mode$ CounterAdded line without CounterAmount$
// ("Whenever a +1/+1 counter is put on CARDNAME" -- Fathom Mage) is the
// per-counter reading, even when one effect puts the whole batch at once: the
// engine emits ONE CounterChange carrying the batch in Amount, and the queue
// site (checkFaceTriggers) must queue one instance per counter.
//
// The "one or more" shape (Mode$ CounterAddedOnce) and the crossing shape
// (CounterAmount$ EQ<n>, "when the tenth counter is put") stay ONE instance.
// The removal modes follow the same split (Mode$ CounterRemoved per counter
// vs CounterRemovedOnce / NewCounterAmount$ once).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ds2CounterCard is the synthetic form of Fathom Mage's line: a plain
// per-counter CounterAdded trigger with a body, on a battlefield creature.
const ds2CounterCard = "Name:Per-Counter Watcher\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n" +
	"T:Mode$ CounterAdded | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ P1P1 | Execute$ TrigDraw | OptionalDecider$ You\n" +
	"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1\n"

// TestPlainCounterAddedQueuesOnePerCounter pins the fix: three +1/+1 counters
// in ONE CounterChange event queue three instances of a plain CounterAdded
// line, each carrying the batch-of-one amount, and a body run three times.
func TestPlainCounterAddedQueuesOnePerCounter(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoard(t, e, 0, ds2CounterCard)
	// Precondition: the line under test is the plain per-counter shape, on
	// the battlefield where its TriggerZones$ admits it.
	trig := e.G.Obj(id).Face().Triggers[0]
	if trig.Mode != "CounterAdded" || trig.Params["CounterAmount"] != "" {
		t.Fatalf("synthetic trigger = %q / CounterAmount$ %q, want a plain CounterAdded line",
			trig.Mode, trig.Params["CounterAmount"])
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("source is not on the battlefield")
	}
	// Precondition: the opt-out branches of the ask would make a queued
	// instance vanish, so prove the seat has cards to draw.
	if len(e.G.Zone(state.ZLibrary, 0)) == 0 {
		t.Fatal("library is empty; the draw body could not run")
	}
	e.pendingTriggers = nil

	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 3})
	if got := len(e.pendingTriggers); got != 3 {
		t.Fatalf("pendingTriggers after one 3-counter event = %d, want 3 (CR 603.2c, one per counter)", got)
	}
	for i, pt := range e.pendingTriggers {
		// Each instance carries the batch-of-one amount, so a body reading
		// TriggerCount$Amount reads one counter, not the whole batch.
		if got := pt.Ctx.TriggerContext.TriggerAmount; got != 1 {
			t.Fatalf("instance %d TriggerCount$Amount = %d, want 1", i, got)
		}
		if pt.Source != id {
			t.Fatalf("instance %d source = %d, want %d", i, pt.Source, id)
		}
	}
	// A single-counter event still queues exactly one.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("pendingTriggers after a 1-counter event = %d, want 1", got)
	}
}

// TestPlainCounterAddedDrawsOneCardPerCounter drives the per-counter reading
// end to end on the same synthetic Fathom Mage line: one batch of four
// counters draws four cards, one per independently resolving instance.
func TestPlainCounterAddedDrawsOneCardPerCounter(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoard(t, e, 0, ds2CounterCard)
	if got := e.G.Obj(id).Face().Triggers[0].Params["CounterAmount"]; got != "" {
		t.Fatalf("line carries CounterAmount$ %q, want a plain per-counter line", got)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	if len(e.G.Zone(state.ZLibrary, 0)) < 4 {
		t.Fatalf("library holds %d cards; four draws cannot resolve", len(e.G.Zone(state.ZLibrary, 0)))
	}
	// The batch's four instances are four simultaneous triggered abilities of
	// one controller, so the CR 603.3b order question is posed exactly as it
	// would be for four separately-queued triggers; every ask (order,
	// optional election) is answered here, then the stack drains.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 4})
	if got := len(e.pendingTriggers); got != 4 {
		t.Fatalf("pendingTriggers after a 4-counter batch = %d, want 4", got)
	}
	for n := 0; n < 100 && (len(e.G.Stack) > 0 || len(e.pendingTriggers) > 0); n++ {
		if d := e.Pending(); d != nil {
			switch d.Kind {
			case decision.KTriggerOrder:
				choices := make([]int, 0, d.Max)
				for i := 0; i < d.Max && i < len(d.Options); i++ {
					choices = append(choices, d.Options[i].Index)
				}
				submitChoices(t, e, choices...)
			case decision.KPriority:
				// The drain finished a trigger and handed priority back; pass.
				submitPass(t, e)
			case decision.KTriggerOptional:
				// Each instance's own OptionalDecider$ You election.
				if d.Max != 1 {
					t.Fatalf("optional election Max = %d, want 1", d.Max)
				}
				submitChoices(t, e, d.Options[0].Index)
			case decision.KChoose:
				// Every KChoose here is the instance's optional election
				// (OptionalDecider$ You): take it.
				if d.Max != 1 {
					t.Fatalf("optional election Max = %d, want 1", d.Max)
				}
				submitChoices(t, e, d.Options[0].Index)
			default:
				t.Fatalf("unexpected decision %v during the batch", d.Kind)
			}
			continue
		}
		e.putTriggersOnStack()
		if len(e.G.Stack) > 0 {
			e.resolveTop()
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack did not drain: %d object(s) left", len(e.G.Stack))
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+4 {
		t.Fatalf("hand after a 4-counter batch = %d, want %d (one draw per counter)", got, handBefore+4)
	}
}

// TestCounterAddedOnceAndCrossingQueueOne pins the two shapes that must NOT
// multiply: Mode$ CounterAddedOnce ("one or more counters") and the
// CounterAmount$ EQ crossing line ("when the tenth counter is put").
func TestCounterAddedOnceAndCrossingQueueOne(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	onceSrc := "Name:Once Watcher\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n" +
		"T:Mode$ CounterAddedOnce | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ P1P1 | Execute$ TrigDraw\n" +
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1\n"
	onceID := onBoard(t, e, 0, onceSrc)

	// Precondition: the line really is the "Once" mode, so a pass cannot come
	// from having built the plain per-counter line by accident.
	if got := e.G.Obj(onceID).Face().Triggers[0].Mode; got != "CounterAddedOnce" {
		t.Fatalf("trigger mode = %q, want CounterAddedOnce", got)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: onceID, Counter: "P1P1", Amount: 4})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("CounterAddedOnce after a 4-counter event = %d instance(s), want 1", got)
	}
	// The Once body still reads the whole batch, not one counter.
	if got := e.pendingTriggers[0].Ctx.TriggerContext.TriggerAmount; got != 4 {
		t.Fatalf("CounterAddedOnce TriggerCount$Amount = %d, want 4 (the batch size)", got)
	}

	crossSrc := "Name:Tenth Watcher\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n" +
		"T:Mode$ CounterAdded | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ P1P1 | CounterAmount$ EQ10 | Execute$ TrigDraw\n" +
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1\n"
	crossID := onBoard(t, e, 0, crossSrc)
	trig := e.G.Obj(crossID).Face().Triggers[0]
	if trig.Mode != "CounterAdded" || trig.Params["CounterAmount"] != "EQ10" {
		t.Fatalf("synthetic crossing trigger = %q / %q, want CounterAdded / EQ10",
			trig.Mode, trig.Params["CounterAmount"])
	}
	// The crossing batch (8 -> 11) fires once, on the crossing, not 3 times.
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: crossID, Counter: "P1P1", Amount: 11})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("CounterAmount$ EQ10 crossing batch = %d instance(s), want 1", got)
	}
}

// TestPlainCounterRemovedQueuesOnePerCounter pins the removal mirror on the
// real corpus carrier Protean Hydra ("Whenever a +1/+1 counter is removed from
// CARDNAME, ..."): two counters removed in one event queue two instances,
// while Mode$ CounterRemovedOnce and a NewCounterAmount$ line stay one.
func TestPlainCounterRemovedQueuesOnePerCounter(t *testing.T) {
	t.Parallel()
	hydra := mshCorpusCard(t, "Protean Hydra")
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, hydra)

	// Precondition: the real card carries the plain per-counter removal line.
	found := false
	for _, tr := range e.G.Obj(id).Face().Triggers {
		if tr.Mode == "CounterRemoved" && tr.Params["NewCounterAmount"] == "" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Protean Hydra carries no plain Mode$ CounterRemoved line")
	}
	// Precondition: the counters really are there to be removed, so the
	// removal event below is a real one.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 3})
	if got := e.G.Obj(id).Counter("P1P1"); got != 3 {
		t.Fatalf("Protean Hydra P1P1 counters = %d, want 3 before the removal", got)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: -2})
	if got := len(e.pendingTriggers); got != 2 {
		t.Fatalf("plain CounterRemoved after a 2-counter removal = %d instance(s), want 2", got)
	}
	for i, pt := range e.pendingTriggers {
		if pt.Source != id {
			t.Fatalf("removal instance %d source = %d, want %d", i, pt.Source, id)
		}
	}

	// CounterRemovedOnce stays one instance, carrying the whole batch.
	onceSrc := "Name:Once Untwister\nManaCost:1 U\nTypes:Creature Wizard\nPT:1/1\nOracle:x\n" +
		"T:Mode$ CounterRemovedOnce | ValidCard$ Card.Self | TriggerZones$ Battlefield | CounterType$ P1P1 | Execute$ TrigDraw\n" +
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 1\n"
	onceID := onBoard(t, e, 0, onceSrc)
	if got := e.G.Obj(onceID).Face().Triggers[0].Mode; got != "CounterRemovedOnce" {
		t.Fatalf("trigger mode = %q, want CounterRemovedOnce", got)
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: onceID, Counter: "P1P1", Amount: 2})
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.CounterChange, Obj: onceID, Counter: "P1P1", Amount: -2})
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("CounterRemovedOnce after a 2-counter removal = %d instance(s), want 1", got)
	}
	if got := e.pendingTriggers[0].Ctx.TriggerContext.TriggerAmount; got != 2 {
		t.Fatalf("CounterRemovedOnce TriggerCount$Amount = %d, want 2 (the batch magnitude)", got)
	}
}
