package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestArrangeNoHostStandInKeepsOrder is the Ruling J3 leaf: a host that
// cannot ask -- the effects-package test double, whose Ask always returns
// false -- falls back to the deterministic stand-in of keeping the existing
// order (pile A = the offered options in offered order), and the resolution
// completes rather than wedging on a decision nobody can answer.
//
// The stand-in is narrower than the card text ("put them back in any
// order"): it reorders nothing. The LibraryOrder event it still emits
// carries the full, unchanged order -- the same event the host-answering
// path emits, so the primitive is a single mechanism for both.
func TestArrangeNoHostStandInKeepsOrder(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:C\nTypes:Creature\nPT:1/1\nOracle:x\n")
	ids := fillLibrary(h.g, 0, card, 5)
	before := append([]state.ObjID(nil), h.g.Zone(state.ZLibrary, 0)...)
	if len(ids) != 5 {
		t.Fatalf("set up %d library cards, want 5", len(ids))
	}

	sa := sa(t, "SP$ RearrangeTopOfLibrary | Defined$ You | NumCards$ 3")
	// fakeHost.Ask returns false (the double cannot ask), and the Ctx names
	// seat 0 as the library's owner/controller.
	Resolve(h, &Ctx{Source: 1, Controller: 0}, sa)

	after := h.g.Zone(state.ZLibrary, 0)
	if len(after) != len(before) {
		t.Fatalf("library size %d -> %d after the no-host rearrange", len(before), len(after))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("no-host stand-in reordered the library at index %d: %v -> %v; the stand-in must keep the existing order", i, before[i], after[i])
		}
	}

	// The LibraryOrder event the stand-in emits carries the full, unchanged
	// order, and the resolution completes (nothing suspended, no panic).
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.LibraryOrder {
			found = true
			if ev.Player != 0 || !ev.Secret {
				t.Fatalf("stand-in LibraryOrder event must name the owner and be Secret: %+v", ev)
			}
			if len(ev.IDs) != len(before) {
				t.Fatalf("stand-in LibraryOrder carried %d ids, want the full %d-card order", len(ev.IDs), len(before))
			}
		}
	}
	if !found {
		t.Fatal("no LibraryOrder event emitted by the no-host stand-in")
	}
	if h.Suspended() {
		t.Fatal("the effects test double reported a suspension; a no-host host must not wedge")
	}
}
