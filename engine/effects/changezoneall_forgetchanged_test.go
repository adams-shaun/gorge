package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// forgetChangedSweep remembers three graveyard cards, then resolves a
// ChangeZoneAll that sweeps only the first two (ChangeType$ names them) with
// the given extra parameters. It returns the moved pair, the kept card, the
// source, the ctx and the number of forget-remembered events the sweep emitted.
func forgetChangedSweep(t *testing.T, extra string) (h *fakeHost, moved []*state.Object, kept *state.Object, src *state.Object, c *Ctx, forgets int) {
	t.Helper()
	h, src, cs := forgetFixtureHost(t, "MoveA", "MoveB", "KeepC")
	moved, kept = cs[:2], cs[2]
	var ids []state.ObjID
	for _, o := range cs {
		o.Zone = state.ZGraveyard
		ids = append(ids, o.ID)
	}
	h.g.SetZone(state.ZGraveyard, 0, ids)
	seedRemembered(h, src, ids...)
	if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, ids) {
		t.Fatalf("precondition: persistent remembered = %v, want %v", got, ids)
	}
	c = &Ctx{Source: src.ID, Controller: 0,
		Remembered: []state.Target{{Obj: ids[0]}, {Obj: ids[1]}, {Obj: ids[2]}}}
	parsed := sa(t, "DB$ ChangeZoneAll | Origin$ Graveyard | Destination$ Exile | ChangeType$ Card.namedMoveA,Card.namedMoveB"+extra)
	if parsed.API != "ChangeZoneAll" {
		t.Fatalf("precondition: fixture SA parsed as %q, want ChangeZoneAll", parsed.API)
	}
	before := len(h.log)
	Resolve(h, c, parsed)
	for _, e := range h.log[before:] {
		if e.Kind == events.Choose && e.Obj == src.ID && e.Counter == "forget-remembered" {
			forgets++
		}
	}
	for _, o := range moved {
		if h.g.Obj(o.ID).Zone != state.ZExile {
			t.Fatalf("precondition: %q did not move: zone = %s", o.Face().Name, h.g.Obj(o.ID).Zone)
		}
	}
	if z := h.g.Obj(kept.ID).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition: the unmatched card moved: zone = %s", z)
	}
	return h, moved, kept, h.g.Obj(src.ID), c, forgets
}

// TestChangeZoneAllForgetChangedDropsMovedCards: ForgetChanged$ True removes
// each moved card from both halves of the remembered state (Over the Top).
func TestChangeZoneAllForgetChangedDropsMovedCards(t *testing.T) {
	_, _, kept, src, c, forgets := forgetChangedSweep(t, " | ForgetChanged$ True")
	if forgets != 2 {
		t.Errorf("forget-remembered events = %d, want exactly 2", forgets)
	}
	if got := rememberedIDs(src.Remembered); !sameIDs(got, []state.ObjID{kept.ID}) {
		t.Errorf("persistent remembered = %v, want only %v", got, kept.ID)
	}
	if got := rememberedIDs(c.Remembered); !sameIDs(got, []state.ObjID{kept.ID}) {
		t.Errorf("ctx remembered = %v, want only %v", got, kept.ID)
	}
}

// TestChangeZoneAllForgetChangedAbsentKeepsSet: without the key the same sweep
// forgets nothing.
func TestChangeZoneAllForgetChangedAbsentKeepsSet(t *testing.T) {
	_, moved, kept, src, c, forgets := forgetChangedSweep(t, "")
	if forgets != 0 {
		t.Errorf("forget-remembered events = %d, want 0", forgets)
	}
	want := []state.ObjID{moved[0].ID, moved[1].ID, kept.ID}
	if got := rememberedIDs(src.Remembered); !sameIDs(got, want) {
		t.Errorf("persistent remembered = %v, want %v", got, want)
	}
	if got := rememberedIDs(c.Remembered); !sameIDs(got, want) {
		t.Errorf("ctx remembered = %v, want %v", got, want)
	}
}

// TestChangeZoneAllRememberThenForget: with both riders the moved cards end up
// not remembered -- Forge's order is remember, then forget.
func TestChangeZoneAllRememberThenForget(t *testing.T) {
	_, _, kept, src, c, forgets := forgetChangedSweep(t, " | RememberChanged$ True | ForgetChanged$ True")
	if forgets != 2 {
		t.Errorf("forget-remembered events = %d, want exactly 2", forgets)
	}
	if got := rememberedIDs(src.Remembered); !sameIDs(got, []state.ObjID{kept.ID}) {
		t.Errorf("persistent remembered = %v, want only %v", got, kept.ID)
	}
	if got := rememberedIDs(c.Remembered); !sameIDs(got, []state.ObjID{kept.ID}) {
		t.Errorf("ctx remembered = %v, want only %v", got, kept.ID)
	}
}
