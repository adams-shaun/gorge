package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDigUntilEnteringRandomOrderRestReturnsToTheBottom pins the
// battlefield-entering dig's revealed-rest return: with FoundDestination$
// Battlefield and RevealRandomOrder$ True at the absent
// RevealedLibraryPosition$ (Kethek, Crucible Goliath's shape — the Oracles'
// "put the rest on the bottom of your library in a random order"), the rest
// does NOT stay in place: the Fisher-Yates draws once per rest card and the
// bottom return is a real library-to-library move, so the rest lands at the
// bottom (with the test host's Rand the shuffle is the identity, so the
// order is the return order) while the cards after the found card keep
// their places. The stay-in-place Note is a non-entering dig's marker and
// must not appear.
func TestDigUntilEnteringRandomOrderRestReturnsToTheBottom(t *testing.T) {
	h, ids := digUntilFixture(t)
	draws := h.n
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Battlefield | RevealedDestination$ Library | RevealRandomOrder$ True"))
	// The found Aura entered and attached (the fixture's Bear is the bearer).
	bearer := h.g.Zone(state.ZBattlefield, 0)[0]
	if o := h.g.Obj(ids[1]); o.Zone != state.ZBattlefield || o.AttachedTo != bearer {
		t.Fatalf("found card zone/attach = %s/%d, want battlefield/%d", o.Zone, o.AttachedTo, bearer)
	}
	// The revealed rest (the first land) is at the BOTTOM; the cards after
	// the found card keep their order on top.
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 3 || lib[0] != ids[2] || lib[1] != ids[3] || lib[2] != ids[0] {
		t.Fatalf("library = %v, want [%d %d %d] — tail on top, revealed rest at the bottom", lib, ids[2], ids[3], ids[0])
	}
	// The Fisher-Yates drew once per rest card (one card here); with the
	// zero-draw stay-in-place arm the count would not have moved.
	if h.n-draws != 1 {
		t.Fatalf("Rand draws %d -> %d, want 1 (the seeded random order)", draws, h.n)
	}
	for i := range h.log {
		e := &h.log[i]
		if e.Kind == events.Note && strings.Contains(e.Text, "stay-in-place") {
			t.Fatalf("entering dig emitted the stay-in-place Note: %+v", e)
		}
	}
}

// TestDigUntilNonEnteringRandomOrderRestStaysInPlace is the control: a dig
// whose found card does NOT enter the battlefield (FoundDestination$ Exile)
// keeps the stay-in-place arm — the rest keeps its place with no move and no
// draw, behind the one loud Note.
func TestDigUntilNonEnteringRandomOrderRestStaysInPlace(t *testing.T) {
	h, ids := digUntilFixture(t)
	draws := h.n
	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Exile | RevealedDestination$ Library | RevealRandomOrder$ True"))
	if o := h.g.Obj(ids[1]); o.Zone != state.ZExile {
		t.Fatalf("found card zone = %s, want exile", o.Zone)
	}
	// The revealed rest kept its place at the top; the library closed up
	// behind the exiled found card.
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 3 || lib[0] != ids[0] || lib[1] != ids[2] || lib[2] != ids[3] {
		t.Fatalf("library = %v, want [%d %d %d] — revealed rest stayed in place", lib, ids[0], ids[2], ids[3])
	}
	if h.n != draws {
		t.Fatalf("Rand draws %d -> %d, want none", draws, h.n)
	}
	noted := false
	for i := range h.log {
		e := &h.log[i]
		if e.Kind == events.Note && strings.Contains(e.Text, "stay-in-place") {
			noted = true
		}
	}
	if !noted {
		t.Fatal("no stay-in-place Note for the stay-in-place return")
	}
}
