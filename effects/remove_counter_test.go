package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The RemoveCounter primitive (Forge's RemoveCounterEffect, 202 raw corpus
// lines over 195 files): the single-target counter removal the corpus uses
// overwhelmingly as a chained DB$ sub-ability. Before this file's fix every
// such line fell into the generic "unimplemented API" Note and removed
// nothing; Prize Pig's "remove those counters and untap it" payoff was the
// live symptom. These tests pin the implemented core shapes and keep the
// exotic shapes loud, the same pattern the RemoveCounterAll tests above
// counters.go follow.

func TestRemoveCounterRemovesNamedKindLiteral(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 3)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "SP$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 2"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 1 {
		t.Fatalf("myBear P1P1 = %d, want 1 after removing 2 of 3", got)
	}
}

// Defined absent and no ValidTgts$: the source default (Forge's "an ability
// that names none acts on its source").
func TestRemoveCounterDefaultsToTheSource(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("CHARGE", 2)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]}, sa(t, "DB$ RemoveCounter | CounterType$ CHARGE | CounterNum$ 1"))
	if got := g.Obj(ids["myBear"]).Counter("CHARGE"); got != 1 {
		t.Fatalf("myBear CHARGE = %d, want 1 (no Defined$ acts on the source)", got)
	}
}

func TestRemoveCounterNumAllRemovesEveryCounterOfTheKind(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("RIBBON", 3)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ RIBBON | CounterNum$ All"))
	if got := g.Obj(ids["myBear"]).Counter("RIBBON"); got != 0 {
		t.Fatalf("myBear RIBBON = %d, want 0 (CounterNum$ All)", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == ids["myBear"] && ev.Amount != -3 {
			t.Fatalf("CounterChange Amount = %d, want -3 (the event must not overstate)", ev.Amount)
		}
	}
}

func TestRemoveCounterCounterTypeAllRemovesEveryKind(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 2)
	g.Obj(ids["myBear"]).AddCounter("CHARGE", 4)
	g.Obj(ids["myBear"]).AddCounter("ICE", 1)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ All | CounterNum$ 2"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 = %d, want 0 (CounterType$ All removes every kind)", got)
	}
	if got := g.Obj(ids["myBear"]).Counter("CHARGE"); got != 2 {
		t.Fatalf("CHARGE = %d, want 2 (4 held, 2 removed)", got)
	}
	if got := g.Obj(ids["myBear"]).Counter("ICE"); got != 0 {
		t.Fatalf("ICE = %d, want 0 (1 held, clamped to 1 removed)", got)
	}
}

func TestRemoveCounterTargetedViaCtxTargets(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 2)
	g.Obj(ids["theirBig"]).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Targets: []state.Target{{Obj: ids["myBear"]}}},
		sa(t, "SP$ RemoveCounter | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ 1"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 1 {
		t.Fatalf("targeted myBear P1P1 = %d, want 1", got)
	}
	if got := g.Obj(ids["theirBig"]).Counter("P1P1"); got != 2 {
		t.Fatalf("untargeted theirBig P1P1 = %d, want unchanged at 2", got)
	}
}

// Over-removal clamps: the object holds 2, the ask is 5 — the event carries
// the honest -2, the object ends at 0, and nothing else is emitted.
func TestRemoveCounterOverRemovalClampsWithoutOverstating(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 2)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 5"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 = %d, want 0 (AddCounter clamps at zero)", got)
	}
	changes := 0
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == ids["myBear"] {
			changes++
			if ev.Amount != -2 {
				t.Fatalf("CounterChange Amount = %d, want the honest -2", ev.Amount)
			}
		}
	}
	if changes != 1 {
		t.Fatalf("emitted %d CounterChange events, want exactly 1", changes)
	}
}

// An object holding none of the kind emits nothing at all (the zero-batch
// no-op discipline effRemoveCounterAll follows).
func TestRemoveCounterWithNoneOfTheKindEmitsNothing(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1"))
	if len(h.log) != 0 {
		t.Fatalf("emitted %d events for a counter the object does not hold, want 0: %+v", len(h.log), h.log)
	}
}

func TestRemoveCounterPlayerTargetRemovesPlayerCounters(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "E", Amount: 3})
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ RemoveCounter | CounterType$ E | CounterNum$ 2 | Defined$ Player"))
	if got := g.Players[0].Counter("E"); got != 1 {
		t.Fatalf("seat 0 E counters = %d, want 1", got)
	}
}

// RememberRemoved$ True records one remembered entry per removed counter on
// the source's persistent (event-backed) list — the encoding Count$RememberedSize
// (Prize Pig's untap gate) reads. Nothing removed remembers nothing.
func TestRemoveCounterRememberRemovedRecordsOneEntryPerCounter(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 3)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ All | RememberRemoved$ True"))
	if got := len(g.Obj(ids["myBear"]).Remembered); got != 3 {
		t.Fatalf("source remembered size = %d, want one entry per removed counter (3)", got)
	}
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 = %d, want 0", got)
	}
	// The no-op half: nothing of the kind held, RememberRemoved$ set — the
	// remembered list stays empty too.
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ All | RememberRemoved$ True"))
	if got := len(g.Obj(ids["myBear"]).Remembered); got != 3 {
		t.Fatalf("remembered size after a no-op removal = %d, want still 3", got)
	}
}

// The exotic shapes stay LOUD: one Note naming the shape, nothing moves.
func TestRemoveCounterExoticShapesStayLoud(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	g.Obj(ids["myBear"]).AddCounter("P1P1", 2)
	before := len(h.log)
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Defined$ Self | CounterType$ Any | CounterNum$ 1"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 2 {
		t.Fatalf("CounterType$ Any moved counters (%d left), want untouched", got)
	}
	if len(h.log) != before+1 || h.log[len(h.log)-1].Kind != events.Note {
		t.Fatalf("expected exactly one Note for the exotic shape, got %+v", h.log[before:])
	}
	// The second loud family: Choices$ (a mid-resolution pick).
	Resolve(h, &Ctx{Controller: 0, Source: ids["myBear"]},
		sa(t, "DB$ RemoveCounter | Choices$ Creature.YouCtrl+counters_GE1_P1P1 | CounterType$ P1P1 | CounterNum$ 1"))
	if got := g.Obj(ids["myBear"]).Counter("P1P1"); got != 2 {
		t.Fatalf("Choices$ moved counters, want untouched at 2")
	}
}
