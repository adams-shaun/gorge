package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// searchmay1: an object-target ChangeZone that moves cards into a library
// and states Shuffle$ True shuffles -- Turn the Earth is the silent
// (mandatory) carrier. (The library-search fail-to-find shuffle is pinned in
// cr701_search_shuffle_test.go.) The flag-bearing tail is covered by a direct
// effects test; SP-parented DB carriers ask their own graveyard targeting at
// resolution since task spcz1 and reach that tail in live play (engine/rules/
// pins the live path on Put Away and Cathartic Parting).
//
// The corpus scripts are loaded through the gitignored registry; no Forge
// script text is committed.

// shuffleCountFrom counts seat p's Shuffle events in the engine log at or
// after index start.
func shuffleCountFrom(e *Engine, start int, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == p {
			n++
		}
	}
	return n
}

// TestTurnTheEarthObjectPathShufflesIntoLibrary is half (b) on a real corpus
// card: Turn the Earth's `DB$ ChangeZone | Origin$ Graveyard | Destination$
// Library | Shuffle$ True` moves the chosen graveyard card into its owner's
// library and shuffles there. Before the fix the object path shuffled
// nothing, so the card was silently appended without a shuffle.
func TestTurnTheEarthObjectPathShufflesIntoLibrary(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Turn the Earth")
	// The graveyard card: a creature card owned by seat 0.
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZGraveyard)
	spell := searchMoveByName(t, e, "Turn the Earth", state.ZHand)

	// Precondition: the target is in the zone the Origin$ precondition reads,
	// and it is NOT already in the library (the move and its shuffle must be
	// real).
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: graveyard card zone = %v, want graveyard", o)
	}
	if inZoneOf(e, state.ZLibrary, 0, bear) {
		t.Fatal("precondition: the target is already in the library; the move assertion cannot fail")
	}
	if e.G.Obj(spell) == nil || e.G.Obj(spell).Zone != state.ZHand {
		t.Fatalf("precondition: Turn the Earth zone = %v, want hand", e.G.Obj(spell))
	}

	addMana(t, e, 0, "G")
	start := len(e.L.Events)
	// Cast, answer the target ask with the graveyard bear, then drain: Turn
	// the Earth's resolution asks nothing further (its SubAbility$ GainLife
	// is silent), so castFixture's passUntilNonPriority is the wrong driver.
	d := e.Pending()
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for Turn the Earth: %+v", d.Options)
	}
	submitChoices(t, e, castIdx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after the cast: pending = %+v, want the target ask", d)
	}
	tgtIdx := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			tgtIdx = o.Index
		}
	}
	if tgtIdx < 0 {
		t.Fatalf("the graveyard bear %d was not offered as a target: %+v", bear, d.Options)
	}
	submitChoices(t, e, tgtIdx)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("after resolution: card zone = %v, want library", o)
	}
	if got := shuffleCountFrom(e, start, 0); got != 1 {
		t.Fatalf("seat-0 shuffles after the object-path move = %d, want exactly 1: %+v", got, e.L.Events[start:])
	}
	replayCheck(t, e, cfg)
}

// inZoneOf reports whether id is in zone z for player p.
func inZoneOf(e *Engine, z state.Zone, p state.PlayerID, id state.ObjID) bool {
	for _, v := range e.G.Zone(z, p) {
		if v == id {
			return true
		}
	}
	return false
}
