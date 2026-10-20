package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g09 (ticket b): CR 701.24c/d. "If an effect would cause a player to
// shuffle one or more specific objects into a library, that library is
// shuffled even if none of those objects are in the zone they're expected
// to be in" (701.24c); "... a set of objects into a library, that library is
// shuffled even if there are no objects in that set" (701.24d).

// TestCR70124ObjectPathShufflesWhenTheObjectLeftItsZone: "shuffle it into
// its owner's library" (Defined$ Self) after the card already left the
// graveyard (exiled in response). The card stays in exile, and its owner's
// library is still shuffled exactly once.
func TestCR70124ObjectPathShufflesWhenTheObjectLeftItsZone(t *testing.T) {
	ah := &fx42AskHost{}
	ah.g = state.NewGame(names(2))
	src := ah.g.AddObject(creature(t, "Avatar"), 0)
	src.Zone = state.ZExile
	ah.g.SetZone(state.ZExile, 0, []state.ObjID{src.ID})
	c := &Ctx{Source: src.ID, Controller: 0}
	s := sa(t, "DB$ ChangeZone | Defined$ Self | Origin$ Graveyard | Destination$ Library | Shuffle$ True")
	effChangeZone(ah, c, s)
	if o := ah.g.Obj(src.ID); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the exiled card = %+v, want it still in exile (it was not in the Origin$ zone)", o)
	}
	if got := shuffleEvents(&ah.fakeHost, 0); got != 1 {
		t.Fatalf("owner shuffles = %d, want exactly 1 (CR 701.24c): %+v", got, ah.log)
	}
	if len(ah.asks) != 0 {
		t.Fatalf("a mandatory Shuffle$ True move posed %d ask(s), want none", len(ah.asks))
	}
}

// TestCR70124ChangeZoneAllShufflesEveryScopedLibrary: Mnemonic Nexus's "Each
// player shuffles their graveyard into their library" (Defined$ Player).
// Seat 1's graveyard is empty; its library is still shuffled.
func TestCR70124ChangeZoneAllShufflesEveryScopedLibrary(t *testing.T) {
	h, c := fixtureHost(t)
	gy := h.g.AddObject(creature(t, "Graveyard Bear"), 0)
	gy.Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{gy.ID})
	s := sa(t, "SP$ ChangeZoneAll | Defined$ Player | ChangeType$ Card | Origin$ Graveyard | Destination$ Library | Shuffle$ True")
	effChangeZoneAll(h, c, s)
	if o := h.g.Obj(gy.ID); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("seat 0's graveyard card = %+v, want it in the library", o)
	}
	for p, want := range []int{1, 1} {
		if got := shuffleEvents(h, state.PlayerID(p)); got != want {
			t.Fatalf("seat %d shuffles = %d, want %d (CR 701.24d: an empty set still shuffles): %+v", p, got, want, h.log)
		}
	}
}

// TestCR70124ChangeZoneAllEmptyDefinedGraveyardStillShuffles: "Shuffle your
// graveyard into your library" (Defined$ You) with an empty graveyard.
func TestCR70124ChangeZoneAllEmptyDefinedGraveyardStillShuffles(t *testing.T) {
	h, c := fixtureHost(t)
	s := sa(t, "SP$ ChangeZoneAll | Defined$ You | ChangeType$ Card | Origin$ Graveyard | Destination$ Library | Shuffle$ True")
	effChangeZoneAll(h, c, s)
	if got := shuffleEvents(h, 0); got != 1 {
		t.Fatalf("seat 0 shuffles = %d, want exactly 1 (CR 701.24d): %+v", got, h.log)
	}
	if got := shuffleEvents(h, 1); got != 0 {
		t.Fatalf("seat 1 shuffles = %d, want 0 (not in scope)", got)
	}
}

// TestChangeZoneAllUnscopedShufflesOnlyReceivingLibraries is the guard,
// green today: the unscoped all-players default (Card.YouOwn, no Defined$)
// moves seat 0's card and shuffles seat 0 only, never every library.
func TestChangeZoneAllUnscopedShufflesOnlyReceivingLibraries(t *testing.T) {
	h, c := fixtureHost(t)
	mine := h.g.AddObject(creature(t, "Mine"), 0)
	mine.Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{mine.ID})
	theirs := h.g.AddObject(creature(t, "Theirs"), 1)
	theirs.Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 1, []state.ObjID{theirs.ID})
	s := sa(t, "SP$ ChangeZoneAll | ChangeType$ Card.YouOwn | Origin$ Graveyard | Destination$ Library | Shuffle$ True")
	effChangeZoneAll(h, c, s)
	if got := shuffleEvents(h, 0); got != 1 {
		t.Fatalf("seat 0 shuffles = %d, want 1", got)
	}
	if got := shuffleEvents(h, 1); got != 0 {
		t.Fatalf("seat 1 shuffles = %d, want 0 (nothing of seat 1's was swept)", got)
	}
	if o := h.g.Obj(theirs.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("seat 1's card = %+v, want it still in the graveyard", o)
	}
}
