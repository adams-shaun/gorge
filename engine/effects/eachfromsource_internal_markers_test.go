package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPutCounterEachFromSourceIgnoresInternalMarkers pins the CR 122.1 read
// on all three rungs of the EachFromSource LKI ladder: the engine's Shield
// and Deathtouched status markers are not counters, so "put those counters
// on ..." must not hand a recipient a marker it never earned. Real counters
// must keep copying (the positive control at the end).
func TestPutCounterEachFromSourceIgnoresInternalMarkers(t *testing.T) {
	sa := &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
		"Defined":        "Targeted",
		"CounterType":    "EachFromSource",
		"EachFromSource": "Remembered",
		"CounterNum":     "1",
		"RememberPut":    "True",
	}}

	run := func(t *testing.T, marker string, seed func(src *state.Object, c *Ctx)) {
		t.Helper()
		g, ids := board(t)
		h := &fakeHost{g: g}
		src := g.Obj(ids["myBear"])
		dst := g.Obj(ids["theirBig"])
		c := &Ctx{Source: src.ID, Controller: 0,
			Targets:    []state.Target{{Obj: dst.ID}},
			Remembered: []state.Target{{Obj: src.ID}}}
		seed(src, c)
		if got := dst.Counter(marker); got != 0 {
			t.Fatalf("precondition: recipient starts with %s = %d, want 0", marker, got)
		}
		base := len(c.Remembered)

		putCounterEachFromSource(h, c, sa, false, "Remembered")

		if got := dst.Counter(marker); got != 0 {
			t.Fatalf("marker %s copied: recipient %s = %d, want 0 (CR 122.1)", marker, marker, got)
		}
		for _, ev := range h.log {
			if ev.Kind == events.CounterChange && ev.Obj == dst.ID && state.InternalCounterMarker(ev.Counter) {
				t.Fatalf("emitted a CounterChange for internal marker %s: %+v", ev.Counter, ev)
			}
		}
		if len(c.Remembered) != base {
			t.Fatalf("recipient remembered after copying only a marker: %+v", c.Remembered[base:])
		}
	}

	t.Run("live rung", func(t *testing.T) {
		run(t, "Shield", func(src *state.Object, _ *Ctx) {
			src.Counters = []state.Counter{{Kind: "Shield", N: 1}}
			if got := src.Counter("Shield"); got != 1 {
				t.Fatalf("precondition: live source Shield = %d, want 1", got)
			}
		})
	})

	t.Run("LKI rung", func(t *testing.T) {
		run(t, "Deathtouched", func(src *state.Object, c *Ctx) {
			src.Counters = nil
			c.LKI = &state.Object{ID: src.ID, Counters: []state.Counter{{Kind: "Deathtouched", N: 1}}}
			if got := c.LKI.Counter("Deathtouched"); got != 1 {
				t.Fatalf("precondition: LKI source Deathtouched = %d, want 1", got)
			}
		})
	})

	t.Run("sacrificed rung", func(t *testing.T) {
		run(t, "Shield", func(src *state.Object, c *Ctx) {
			src.Counters = nil
			c.Sacrificed = []state.SacrificedInfo{{Obj: src.ID,
				Counters: []state.Counter{{Kind: "Shield", N: 1}}}}
			if len(c.Sacrificed) != 1 || len(c.Sacrificed[0].Counters) != 1 {
				t.Fatalf("precondition: sacrificed snapshot not seeded: %+v", c.Sacrificed)
			}
		})
	})

	t.Run("positive control copies real counters", func(t *testing.T) {
		g, ids := board(t)
		h := &fakeHost{g: g}
		src := g.Obj(ids["myBear"])
		dst := g.Obj(ids["theirBig"])
		src.Counters = []state.Counter{{Kind: "P1P1", N: 2}, {Kind: "CHARGE", N: 1}}
		if src.Counter("P1P1") != 2 || src.Counter("CHARGE") != 1 {
			t.Fatalf("precondition: source counters not seeded: %+v", src.Counters)
		}
		c := &Ctx{Source: src.ID, Controller: 0,
			Targets:    []state.Target{{Obj: dst.ID}},
			Remembered: []state.Target{{Obj: src.ID}}}
		base := len(c.Remembered)

		putCounterEachFromSource(h, c, sa, false, "Remembered")

		if got := dst.Counter("P1P1"); got != 2 {
			t.Fatalf("positive control: recipient P1P1 = %d, want 2", got)
		}
		if got := dst.Counter("CHARGE"); got != 1 {
			t.Fatalf("positive control: recipient CHARGE = %d, want 1", got)
		}
		if len(c.Remembered) != base+1 || c.Remembered[base].Obj != dst.ID {
			t.Fatalf("positive control: recipient not remembered: %+v", c.Remembered[base:])
		}
	})
}
