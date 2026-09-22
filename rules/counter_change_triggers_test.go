package rules

// The counter-change trigger modes: trig:CounterRemoved, trig:CounterRemovedOnce
// and trig:CounterAddedOnce, pinned on the three real corpus cards the brief
// names (Dinosaurs on a Spaceship, Regenerations Restored, Kate Stewart).
//
// All three are the SAME event shape CounterAdded already reads: the engine
// emits ONE CounterChange per placement/removal batch, with the whole batch in
// Amount (positive for a put, negative for a removal). "Once per batch" is
// therefore the event granularity itself -- there is no per-turn latch like
// TokenCreatedOnce ("the first time each turn"), because these modes' oracle
// text is "one or more counters", not "each turn". The Once variants are
// distinguished only by the referent they capture: a CounterRemovedOnce body
// reads the removed magnitude through TriggerCount$Amount (Chandra, Fire
// Artisan's "deals that much damage"), which the Referents case supplies as a
// POSITIVE number.
//
// The tests drive events.CounterChange directly (the way the existing
// counter_removed_trigger_test.go / counteraddedonce_test.go do), because
// Vanishing -- the engine-side source of a battlefield TIME removal for
// Regenerations Restored -- is not implemented, and the purpose here is the
// trigger mode, not the counter source. TestDinosaursTokenPerTimeCounterRemoved
// additionally asserts the real Suspend upkeep decrement routes through an
// observable CounterChange, so the exile-scoped Dinosaurs trigger can see it.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusTokenEngine builds a corpus-backed two-seat engine with the token
// scripts loaded, so a resolved DB$ Token | TokenScript$ ... body can mint its
// token. combatEngine's deck-only Config carries no Tokens map, which is fine
// for trigger-matching tests but leaves a token body with nothing to mint.
func corpusTokenEngine(t *testing.T) *Engine {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: reg.Tokens}))
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e
}

// TestKateStewartSoldierPerTimeCounterBatch pins trig:CounterAddedOnce on the
// real corpus card Kate Stewart: one CounterChange carrying a whole TIME
// placement batch queues exactly ONE trigger, and its body creates one 1/1
// white Soldier -- once per batch, never once per counter. A second batch
// triggers again (it is not a one-shot), and a batch of a different counter
// kind does not (CounterType$ TIME).
func TestKateStewartSoldierPerTimeCounterBatch(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	kate := onBoardCard(t, e, 0, mshCorpusCard(t, "Kate Stewart"))
	// The trigger's ValidCard$ is Permanent.YouCtrl+inRealZoneBattlefield, so
	// the counters must land on a battlefield permanent Kate's controller
	// owns. Use another battlefield permanent (not self-scoped).
	target := onBoard(t, e, 0, "Name:Time Vault\nTypes:Artifact\nOracle:x\n")

	soldiers := func() int {
		n := 0
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Soldier Token" {
				n++
			}
		}
		return n
	}

	// One batch of three time counters: exactly one trigger.
	e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "TIME", Amount: 3})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after a 3-time-counter batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := soldiers(); got != 1 {
		t.Fatalf("Soldier tokens after a 3-counter batch = %d, want 1 (once per batch, not per counter)", got)
	}

	// A second batch triggers again: the Once contract is per batch, not a
	// game-long one-shot.
	e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "TIME", Amount: 2})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after a second TIME batch = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := soldiers(); got != 2 {
		t.Fatalf("Soldier tokens after two batches = %d, want 2", got)
	}

	// A different counter kind is not "one or more time counters".
	e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "CHARGE", Amount: 4})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("a CHARGE batch queued %d triggers, want 0", len(e.pendingTriggers))
	}
	// Kate herself is a permanent you control, and the trigger is not
	// self-scoped: a TIME batch on Kate also triggers.
	e.emit(events.Event{Kind: events.CounterChange, Obj: kate, Counter: "TIME", Amount: 1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("a TIME batch on Kate herself queued %d triggers, want 1", len(e.pendingTriggers))
	}
}

// TestDinosaursTokenPerTimeCounterRemoved pins trig:CounterRemoved on the real
// corpus card Dinosaurs on a Spaceship: its trigger is scoped
// TriggerZones$ Exile with CounterType$ TIME, so a TIME removal while exiled
// queues one trigger whose body creates a 2/2 red and white Dinosaur with
// flying and haste. The Suspend upkeep decrement -- the engine's own TIME
// removal for an exiled suspended card -- is asserted to route through an
// observable CounterChange, which is what makes the trigger reachable at all.
func TestDinosaursTokenPerTimeCounterRemoved(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	dino := mshCorpusCard(t, "Dinosaurs on a Spaceship")

	// Exiled with suspend provenance and three TIME counters.
	o := e.G.AddObject(dino, 0)
	id := o.ID
	o.Zone = state.ZExile
	o.CastFlags |= state.FlagSuspend
	o.AddCounter("TIME", 3)
	e.G.SetZone(state.ZExile, 0, append(e.G.Zone(state.ZExile, 0), id))

	dinosaurs := func() int {
		n := 0
		for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
			if t2 := e.G.Obj(oid); t2 != nil && t2.Face() != nil && t2.Face().Name == "Dinosaur Token" {
				n++
			}
		}
		return n
	}

	// The engine's own suspend decrement: entering seat 0's upkeep step runs
	// finishEnteredStep, which drains one TIME counter from each suspended
	// card through an observable CounterChange the trigger reads.
	e.G.Active = 0
	e.G.Step = state.StepUpkeep
	e.finishEnteredStep()
	if got := e.G.Obj(id).Counter("TIME"); got != 2 {
		t.Fatalf("TIME counters after the upkeep suspend decrement = %d, want 2 (the decrement must be observable)", got)
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the suspend TIME decrement = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := dinosaurs(); got != 1 {
		t.Fatalf("Dinosaur tokens after one TIME removal = %d, want 1", got)
	}

	// A further removal fires again (per removal, not per card).
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after a second TIME removal = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if got := dinosaurs(); got != 2 {
		t.Fatalf("Dinosaur tokens after two TIME removals = %d, want 2", got)
	}

	// Zone gate: the same trigger on the BATTLEFIELD does not fire -- the card
	// is only a carrier while exiled.
	e2 := corpusTokenEngine(t)
	bid := onBoardCard(t, e2, 0, dino)
	e2.emit(events.Event{Kind: events.CounterChange, Obj: bid, Counter: "TIME", Amount: 1})
	e2.emit(events.Event{Kind: events.CounterChange, Obj: bid, Counter: "TIME", Amount: -1})
	if len(e2.pendingTriggers) != 0 {
		t.Fatalf("a battlefield TIME removal queued %d triggers, want 0 (exile-scoped)", len(e2.pendingTriggers))
	}
}

// TestRegenerationsRestoredScrysOncePerBatch pins trig:CounterRemovedOnce on
// the real corpus card Regenerations Restored: one CounterChange carrying a
// whole TIME removal batch queues exactly ONE trigger (the Once contract), and
// its chained body scrys 1 and gains 1 life. Two separate removals are two
// batches and so two triggers. This is the mode that had NO matcher at all
// before the fix -- the trigger never queued.
func TestRegenerationsRestoredScrysOncePerBatch(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	id := onBoardCard(t, e, 0, mshCorpusCard(t, "Regenerations Restored"))

	// The library must hold enough cards for the scry to have a window; the
	// scry itself poses a real KArrange ask below.
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) < 3 {
		t.Fatalf("setup: seat 0 library has %d cards, want at least 3", len(lib))
	}
	lifeBefore := e.G.Players[0].Life

	// Give it a batch of time counters to remove from.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: 3})
	// That put is not a removal: no CounterRemovedOnce trigger yet.
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("a TIME ADD queued %d removal triggers, want 0", len(e.pendingTriggers))
	}

	// One removal batch of two: exactly one trigger; resolving it scrys 1 and
	// gains 1 life. The scry poses a real KArrange ask (this engine has a
	// host), so the resolution suspends until the ask is answered -- keep
	// nothing on top, which also exercises the chain.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -2})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after a -2 TIME batch = %d, want 1 (once per batch)", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if d := e.Pending(); d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the scry's KArrange ask, got %+v", d)
	}
	submitChoices(t, e)
	if got := e.G.Players[0].Life; got != lifeBefore+1 {
		t.Fatalf("life after the CounterRemovedOnce body = %d, want %d (scry 1, gain 1 life)", got, lifeBefore+1)
	}
	if got := e.G.Obj(id).Counter("TIME"); got != 1 {
		t.Fatalf("TIME counters after the -2 batch = %d, want 1", got)
	}

	// A second removal batch is a new batch and fires again.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after the last TIME removal = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if d := e.Pending(); d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected the scry's KArrange ask on the last removal, got %+v", d)
	}
	submitChoices(t, e)
	if got := e.G.Players[0].Life; got != lifeBefore+2 {
		t.Fatalf("life after the second removal batch = %d, want %d", got, lifeBefore+2)
	}

	// CounterType$ filters: a removal of another kind queues nothing.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: -1})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("a CHARGE removal queued %d triggers, want 0", len(e.pendingTriggers))
	}
}

// TestCounterRemovedOnceReferentsCaptureTheBatch pins the referent binding
// directly: the CounterRemovedOnce case must set TriggerAmount to the POSITIVE
// magnitude of the removal batch (Chandra, Fire Artisan's TriggerCount$Amount)
// and TriggerCard to the permanent the counters left.
func TestCounterRemovedOnceReferentsCaptureTheBatch(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, mshCorpusCard(t, "Regenerations Restored"))
	trig := e.G.Obj(id).Face().Triggers[0]
	if trig.Mode != "CounterRemovedOnce" {
		t.Fatalf("Regenerations Restored trigger[0] mode = %q, want CounterRemovedOnce", trig.Mode)
	}
	ctx := e.triggerReferents(trig, id, events.Event{
		Kind: events.CounterChange, Obj: id, Counter: "TIME", Amount: -3,
	}, nil)
	if ctx.TriggerAmount != 3 {
		t.Fatalf("TriggerAmount = %d, want 3 (the POSITIVE removal magnitude)", ctx.TriggerAmount)
	}
	if ctx.TriggerCard != id {
		t.Fatalf("TriggerCard = %d, want %d (the counter source)", ctx.TriggerCard, id)
	}
}

// TestCounterRemovedOnceIsRegistered: the RegisterNonAPI registration makes
// effects.Supported() report the primitive, so the coverage report/ratchet see
// it.
func TestCounterRemovedOnceIsRegistered(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"trig:CounterRemoved", "trig:CounterRemovedOnce", "trig:CounterAddedOnce"} {
		if !effects.Supported()[mode] {
			t.Errorf("effects.Supported() lacks %s", mode)
		}
	}
}
