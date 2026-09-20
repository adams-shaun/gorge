package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// effMultiplyCounter (Forge's MultiplyCounterEffect) unit leaves. The real
// corpus in-deck carrier -- Lily Bowen, Raging Grandma's upkeep doubling -- is
// pinned end to end in rules/multiply_counter_test.go; these exercise the
// sibling shapes that test does not reach, all on the SAME primitive:
//   - an absent CounterType$ doubles EACH KIND of counter on the object
//     ("double the number of each kind of counter on target permanent",
//     Aetheric Amplifier / Deepglow Skate / The Thing);
//   - a Multiplier$ other than 2 adds (Multiplier-1) x current;
//   - a player target (Defined$ You: "double the number of each kind of
//     counter you have", Aetheric Amplifier's second mode) doubles PLAYER
//     counters through PlayerCounterChange;
//   - a target with no counters emits nothing (a zero add is a no-op).

// countedObject returns a battlefield object controlled by seat 0 carrying
// the given counter kinds (set through the events path, never a direct field
// write). Kinds are an ordered slice, not a map: nothing in this repo ranges a
// map where the order can reach an event.
func countedObject(t *testing.T, h *fakeHost, counters []struct {
	kind string
	n    int32
}) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:Counted\nTypes:Artifact\nOracle:x\n"), 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
	for _, c := range counters {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: c.kind, Amount: c.n})
	}
	return o.ID
}

// ck is one countedObject entry.
type ck = struct {
	kind string
	n    int32
}

func TestMultiplyCounterDoublesEachKindOnObject(t *testing.T) {
	h := newHost(t, 2)
	id := countedObject(t, h, []ck{{"P1P1", 3}, {"CHARGE", 2}})
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}}
	Resolve(h, c, sa(t, "DB$ MultiplyCounter | Defined$ Targeted"))
	o := h.g.Obj(id)
	if got := o.Counter("P1P1"); got != 6 {
		t.Fatalf("P1P1 = %d, want 6 (3 doubled; absent CounterType$ doubles each kind)", got)
	}
	if got := o.Counter("CHARGE"); got != 4 {
		t.Fatalf("CHARGE = %d, want 4 (2 doubled)", got)
	}
}

func TestMultiplyCounterHonoursMultiplier(t *testing.T) {
	h := newHost(t, 2)
	id := countedObject(t, h, []ck{{"P1P1", 3}})
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}}
	Resolve(h, c, sa(t, "DB$ MultiplyCounter | Defined$ Targeted | CounterType$ P1P1 | Multiplier$ 3"))
	if got := h.g.Obj(id).Counter("P1P1"); got != 9 {
		t.Fatalf("P1P1 = %d, want 9 (3 + (3-1)*3)", got)
	}
}

func TestMultiplyCounterDoublesPlayerCounters(t *testing.T) {
	h := newHost(t, 2)
	h.Emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 2})
	c := &Ctx{Controller: 0}
	Resolve(h, c, sa(t, "DB$ MultiplyCounter | Defined$ You | Multiplier$ 2"))
	if got := h.g.Players[0].Counter("POISON"); got != 4 {
		t.Fatalf("player poison = %d, want 4 (2 doubled; Defined$ You player target)", got)
	}
}

func TestMultiplyCounterNoCountersEmitsNothing(t *testing.T) {
	h := newHost(t, 2)
	id := countedObject(t, h, nil)
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: id}}}
	before := len(h.log)
	Resolve(h, c, sa(t, "DB$ MultiplyCounter | Defined$ Targeted | CounterType$ P1P1"))
	if len(h.log) != before {
		t.Fatalf("emitted %d events for a counter-less target, want 0", len(h.log)-before)
	}
}
