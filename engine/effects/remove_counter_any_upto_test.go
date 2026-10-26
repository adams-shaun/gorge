package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRemoveCounterAnyUpTo pins Glissa Sunslayer's third mode:
// `RemoveCounter | CounterType$ Any | CounterNum$ 3 | UpTo$ True`. Before the
// fix CounterType$ Any was an exotic shape, so the mode emitted one Note and
// removed nothing. The controller now picks the kind for each counter removed
// (one at a time, from the kinds still present), up to three in total, and
// Done stops the election early.

type anyUpToAnswer struct {
	kind  string
	label string
}

// anyUpToHost serves a scripted answer sequence through the ask seam, so the
// effects-level test exercises the real per-counter election rather than the
// R-9 stand-in.
type anyUpToHost struct {
	*fakeHost
	answers []anyUpToAnswer
	asks    []*decision.Decision
}

func (h *anyUpToHost) AskCount() uint64 { return 0 }

func (h *anyUpToHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if d.ResumeKind != "counter_kind" {
		// Leave the ordinary target ask to the R-9 fallback (Ctx.Targets).
		return decision.Intent{}, false
	}
	h.asks = append(h.asks, d)
	if len(h.answers) == 0 {
		return decision.Intent{}, false
	}
	a := h.answers[0]
	h.answers = h.answers[1:]
	for _, o := range d.Options {
		if (a.kind != "" && o.Kind == a.kind) || (a.label != "" && o.Label == a.label) {
			return decision.Intent{Choices: []int{o.Index}}, true
		}
	}
	return decision.Intent{}, false
}

func glissaAnyUpToBoard(t *testing.T) (*anyUpToHost, *cards.SA, state.ObjID, state.ObjID) {
	t.Helper()
	card, mode := corpusSA(t, "Glissa Sunslayer", "DBRemove")
	if mode.API != "RemoveCounter" ||
		!strings.EqualFold(mode.Params["CounterType"], "Any") ||
		!strings.EqualFold(mode.Params["CounterNum"], "3") ||
		!strings.EqualFold(mode.Params["UpTo"], "True") {
		t.Fatalf("Glissa's DBRRemove = %s %+v, want RemoveCounter Any/3/UpTo", mode.API, mode.Params)
	}
	h := &anyUpToHost{fakeHost: newHost(t, 2)}
	src := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: state.ZLibrary, To: state.ZBattlefield})
	perm := h.g.AddObject(mkCard(t, "Name:Counter Carrier\nTypes:Artifact\nOracle:x\n"), 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: perm.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: perm.ID, Counter: "P1P1", Amount: 2})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: perm.ID, Counter: "OIL", Amount: 2})
	if o := h.g.Obj(perm.ID); o.Counter("P1P1") != 2 || o.Counter("OIL") != 2 {
		t.Fatalf("precondition: carrier counters = %+v, want two P1P1 and two OIL", o.Counters)
	}
	return h, mode, src.ID, perm.ID
}

func anyUpToCtx(src, perm state.ObjID) *Ctx {
	return &Ctx{Controller: 0, Source: src, Targets: []state.Target{{Obj: perm}}}
}

func anyUpToRemovals(h *anyUpToHost, perm state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange && ev.Obj == perm && ev.Amount < 0 {
			out = append(out, ev)
		}
	}
	return out
}

func assertAnyUpToAsks(t *testing.T, h *anyUpToHost, wantAsks int) {
	t.Helper()
	if len(h.asks) != wantAsks {
		t.Fatalf("asks = %d, want %d: %+v", len(h.asks), wantAsks, h.asks)
	}
	for i, d := range h.asks {
		if d.Kind != decision.KChoose || d.ResumeKind != "counter_kind" || d.Min != 1 || d.Max != 1 {
			t.Fatalf("ask %d = %+v, want a 1-of counter_kind KChoose", i, d)
		}
		done := false
		for _, o := range d.Options {
			if o.Kind == "remove_counter_done" {
				done = true
			}
		}
		if !done {
			t.Fatalf("ask %d has no Done option: %+v", i, d.Options)
		}
	}
}

func TestRemoveCounterAnyUpTo(t *testing.T) {
	t.Run("three counters across the kinds picked", func(t *testing.T) {
		h, mode, src, perm := glissaAnyUpToBoard(t)
		// Drain P1P1 first so the third ask sees a single remaining kind and
		// still offers Done (UpTo$ is a real election even then).
		h.answers = []anyUpToAnswer{{label: "P1P1"}, {label: "P1P1"}, {label: "OIL"}}
		Resolve(h, anyUpToCtx(src, perm), mode)

		if got := h.g.Obj(perm).Counter("P1P1"); got != 0 {
			t.Fatalf("P1P1 = %d, want 0 after two removals", got)
		}
		if got := h.g.Obj(perm).Counter("OIL"); got != 1 {
			t.Fatalf("OIL = %d, want 1 after one removal", got)
		}
		evs := anyUpToRemovals(h, perm)
		if len(evs) != 2 {
			t.Fatalf("removal events = %+v, want one batched event per kind (P1P1 -2, OIL -1)", evs)
		}
		if evs[0].Counter != "P1P1" || evs[0].Amount != -2 {
			t.Fatalf("first removal = %+v, want P1P1 -2 (the whole P1P1 batch)", evs[0])
		}
		if evs[1].Counter != "OIL" || evs[1].Amount != -1 {
			t.Fatalf("second removal = %+v, want OIL -1", evs[1])
		}
		assertAnyUpToAsks(t, h, 3)
		third := h.asks[2]
		if len(third.Options) != 2 || third.Options[0].Label != "OIL" ||
			third.Options[1].Kind != "remove_counter_done" {
			t.Fatalf("third ask = %+v, want OIL + Done (the drained kind is gone)", third.Options)
		}
	})

	t.Run("Done stops early", func(t *testing.T) {
		h, mode, src, perm := glissaAnyUpToBoard(t)
		h.answers = []anyUpToAnswer{{label: "P1P1"}, {label: "OIL"}, {kind: "remove_counter_done"}}
		Resolve(h, anyUpToCtx(src, perm), mode)

		if got := h.g.Obj(perm).Counter("P1P1"); got != 1 {
			t.Fatalf("P1P1 = %d, want 1 after one removal", got)
		}
		if got := h.g.Obj(perm).Counter("OIL"); got != 1 {
			t.Fatalf("OIL = %d, want 1 after one removal", got)
		}
		evs := anyUpToRemovals(h, perm)
		if len(evs) != 2 {
			t.Fatalf("removal events = %+v, want one per kind before Done", evs)
		}
		if evs[0].Counter != "P1P1" || evs[0].Amount != -1 ||
			evs[1].Counter != "OIL" || evs[1].Amount != -1 {
			t.Fatalf("removals = %+v, want P1P1 -1 then OIL -1", evs)
		}
		assertAnyUpToAsks(t, h, 3)
	})

	t.Run("player counters across kinds", func(t *testing.T) {
		h := &anyUpToHost{fakeHost: newHost(t, 2)}
		h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 2})
		h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "RAD", Amount: 1})
		h.answers = []anyUpToAnswer{{label: "POISON"}, {label: "RAD"}}
		Resolve(h, &Ctx{Controller: 0},
			sa(t, "SP$ RemoveCounter | Defined$ Player | CounterType$ Any | CounterNum$ 2 | UpTo$ True"))

		if got := h.g.Players[0].Counter("POISON"); got != 1 {
			t.Fatalf("POISON = %d, want 1 after one removal", got)
		}
		if got := h.g.Players[0].Counter("RAD"); got != 0 {
			t.Fatalf("RAD = %d, want 0 after one removal", got)
		}
		changes := 0
		for _, ev := range h.log {
			if ev.Kind == events.PlayerCounterChange && ev.Player == 0 && ev.Amount < 0 {
				changes++
			}
		}
		if changes != 2 {
			t.Fatalf("player removal events = %d, want 2", changes)
		}
		assertAnyUpToAsks(t, h, 2)
	})
}
