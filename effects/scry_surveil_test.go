package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestScryNoHostStandInKeepsOrder is the Ruling J6.4 leaf for Scry: a host
// that cannot ask (the effects-package test double, whose Ask always returns
// false) falls back to the deterministic stand-in of keeping every card on
// top in its existing order -- pile B empty, nothing to the bottom -- and the
// resolution completes rather than wedging.
func TestScryNoHostStandInKeepsOrder(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:C\nTypes:Creature\nPT:1/1\nOracle:x\n")
	ids := fillLibrary(h.g, 0, card, 5)
	before := append([]state.ObjID(nil), h.g.Zone(state.ZLibrary, 0)...)
	if len(ids) != 5 {
		t.Fatalf("set up %d library cards, want 5", len(ids))
	}

	Resolve(h, &Ctx{Source: 1, Controller: 0}, sa(t, "SP$ Scry | Defined$ You | ScryNum$ 3"))

	after := h.g.Zone(state.ZLibrary, 0)
	if len(after) != len(before) {
		t.Fatalf("library size %d -> %d after the no-host scry", len(before), len(after))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("no-host scry reordered the library at index %d: %v -> %v; the stand-in must keep the existing order", i, before[i], after[i])
		}
	}
	// The stand-in emits a LibraryOrder carrying the full, unchanged order,
	// and no MoveZone (nothing left the library).
	found := false
	for _, ev := range h.log {
		switch ev.Kind {
		case events.LibraryOrder:
			found = true
			if ev.Player != 0 || !ev.Secret {
				t.Fatalf("stand-in LibraryOrder event must name the owner and be Secret: %+v", ev)
			}
			if len(ev.IDs) != len(before) {
				t.Fatalf("stand-in LibraryOrder carried %d ids, want the full %d-card order", len(ev.IDs), len(before))
			}
		case events.MoveZone:
			t.Fatalf("no-host scry emitted a MoveZone %+v; the stand-in must move nothing", ev)
		}
	}
	if !found {
		t.Fatal("no LibraryOrder event emitted by the no-host scry stand-in")
	}
	if h.Suspended() {
		t.Fatal("the effects test double reported a suspension; a no-host host must not wedge")
	}
}

// TestSurveilNoHostStandInPutsNothingInGraveyard is the Ruling J6.4 leaf for
// Surveil: a host that cannot ask leaves every card on top (nothing to the
// graveyard) and emits no MoveZone.
func TestSurveilNoHostStandInPutsNothingInGraveyard(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:C\nTypes:Creature\nPT:1/1\nOracle:x\n")
	fillLibrary(h.g, 0, card, 5)
	before := append([]state.ObjID(nil), h.g.Zone(state.ZLibrary, 0)...)

	Resolve(h, &Ctx{Source: 1, Controller: 0}, sa(t, "SP$ Surveil | Defined$ You | Amount$ 2"))

	after := h.g.Zone(state.ZLibrary, 0)
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("no-host surveil reordered the library at index %d: %v -> %v; the stand-in must keep the existing order", i, before[i], after[i])
		}
	}
	if gy := h.g.Zone(state.ZGraveyard, 0); len(gy) != 0 {
		t.Fatalf("no-host surveil left %d card(s) in the graveyard, want 0 (nothing goes there)", len(gy))
	}
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone {
			t.Fatalf("no-host surveil emitted a MoveZone %+v; the stand-in must move nothing", ev)
		}
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.LibraryOrder {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no LibraryOrder event emitted by the no-host surveil stand-in")
	}
}

// TestSurveilMultiPlayerMarkerOnlyForTheProcessedLibrary is the regression
// for the marker's placement: a multi-player `Defined$ Player` Surveil poses
// only the FIRST library's KArrange ask (the documented multi-library
// Scry/Surveil limitation), so only the player whose arrangement the walk
// actually reached may carry an events.Surveil marker. Before the fix the
// marker was emitted for EVERY acting player up front, which queued
// `T:Mode$ Surveil` triggers for players whose library was never touched
// (an opponent's Whispering Snitch would drain for a surveil that never
// happened). No corpus `Surveil` line carries a multi-player `Defined$`
// (measured: 220 raw Surveil lines, the only `Defined$` values being
// `RememberedLKI`, `Targeted` and `You`), so the shape is synthetic here,
// exactly as the other corpus-unreachable pins are.
func TestSurveilMultiPlayerMarkerOnlyForTheProcessedLibrary(t *testing.T) {
	h := &suspendHost{fakeHost: *newHost(t, 2)}
	card := mkCard(t, "Name:C\nTypes:Creature\nPT:1/1\nOracle:x\n")
	// Both seats hold a full, identity-known library: the precondition the
	// "player 1 was never touched" assertion depends on.
	lib0 := fillLibrary(h.g, 0, card, 3)
	lib1 := fillLibrary(h.g, 1, card, 3)
	if len(lib0) != 3 || len(lib1) != 3 {
		t.Fatalf("setup: libraries are %d and %d cards, want 3 and 3", len(lib0), len(lib1))
	}
	before1 := append([]state.ObjID(nil), h.g.Zone(state.ZLibrary, 1)...)

	ctx := &Ctx{Source: 1, Controller: 0}
	Resolve(h, ctx, sa(t, "SP$ Surveil | Defined$ Player | Amount$ 1"))

	// The first library's KArrange was posed and suspended (the asking host
	// answers true), which is what makes the later player unreachable.
	if h.asked == nil {
		t.Fatal("multi-player surveil posed no decision")
	}
	if h.asked.Player != 0 {
		t.Fatalf("first arrange asked player %d, want 0", h.asked.Player)
	}
	if got := countSurveilMarkers(h); got != 1 {
		t.Fatalf("after the first pass the log carries %d Surveil markers, want 1 (the processed player only)", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Surveil && ev.Player != 0 {
			t.Fatalf("Surveil marker names player %d; only player 0's arrangement was performed", ev.Player)
		}
	}

	// Engine resume: clear Suspended and re-enter with Ctx.Arrange set, the
	// way rules' handleArrange re-drives the SA. The later player is not
	// reached on this pass either (the shared body returns at the top), so no
	// second marker and no arrangement of library 1 may appear.
	h.suspended = false
	ctx.Arrange = true
	Resolve(h, ctx, sa(t, "SP$ Surveil | Defined$ Player | Amount$ 1"))

	after1 := h.g.Zone(state.ZLibrary, 1)
	if len(after1) != len(before1) {
		t.Fatalf("player 1's library size %d -> %d; a later player's library must be untouched", len(before1), len(after1))
	}
	for i := range before1 {
		if after1[i] != before1[i] {
			t.Fatalf("player 1's library reordered at index %d: %v -> %v", i, before1[i], after1[i])
		}
	}
	if got := countSurveilMarkers(h); got != 1 {
		t.Fatalf("after the re-entry the log carries %d Surveil markers, want 1 (no marker for the unprocessed player, none re-emitted on re-entry)", got)
	}
	if h.Suspended() {
		t.Fatal("the host is still suspended after the re-entry pass")
	}
}

// countSurveilMarkers reports how many events.Surveil records the effects
// host's log carries.
func countSurveilMarkers(h *suspendHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Surveil {
			n++
		}
	}
	return n
}
