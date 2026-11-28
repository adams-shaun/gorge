package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 111.8: a token that has left the battlefield can't move to another zone
// or come back onto the battlefield; CR 108.2b: a token isn't a card. Chaos
// Warp's shape: one spell shuffles the target into its owner's library and its
// sub-ability puts the top card onto the battlefield if it is a permanent
// card. With nothing else in that library the top object is the token itself,
// and no state-based action runs between the two moves. The fixture is
// written by hand (never a corpus script).
const warpTopSrc = "Name:Fixture Warp\nManaCost:1 R\nTypes:Instant\n" +
	"A:SP$ ChangeZone | ValidTgts$ Creature.YouCtrl | Origin$ Battlefield | Destination$ Library | Shuffle$ True | SubAbility$ Top\n" +
	"SVar:Top:DB$ Dig | DigNum$ 1 | Defined$ You | Reveal$ True | ChangeNum$ All | ChangeValid$ Permanent | DestinationZone$ Battlefield | DestinationZone2$ Library | LibraryPosition2$ 0\n" +
	"Oracle:x\n"

// warpOntoEmptyLibrary empties seat 0's library, then resolves the fixture
// warp on target, which returns the one object that library then holds.
func warpOntoEmptyLibrary(t *testing.T, e *Engine, target state.ObjID) {
	t.Helper()
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
	}
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("precondition: library holds %d cards, want it empty so the warped object is its top", n)
	}
	addMana(t, e, 0, "CR")
	castFirst(t, e, "cast")
	flickerTarget(t, e, target)
	passUntilStackEmpty(t, e, 20)
}

// warpDigMoves counts the logged moves of id from its library to the
// battlefield: the precondition that the shuffle happened is toLibrary.
func warpDigMoves(e *Engine, id state.ObjID) (toLibrary, back int) {
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind != events.MoveZone || ev.Obj != id {
			continue
		}
		if ev.From == state.ZBattlefield && ev.To == state.ZLibrary {
			toLibrary++
		}
		if ev.From == state.ZLibrary && ev.To == state.ZBattlefield {
			back++
		}
	}
	return toLibrary, back
}

func TestADigDoesNotReturnAWarpedToken(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeckWithTokens(t, 113, warpTopSrc)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	bf := e.G.Zone(state.ZBattlefield, 0)
	tok := bf[len(bf)-1]
	if !e.G.Obj(tok).IsToken || e.G.Obj(tok).Zone != state.ZBattlefield {
		t.Fatalf("precondition: object %d is not a battlefield token (%+v)", tok, e.G.Obj(tok))
	}
	warpOntoEmptyLibrary(t, e, tok)
	toLibrary, back := warpDigMoves(e, tok)
	if toLibrary != 1 {
		t.Fatalf("precondition: the token moved to the library %d times, want 1", toLibrary)
	}
	if back != 0 {
		t.Fatalf("the Dig moved the token back onto the battlefield %d times: CR 111.8 forbids it", back)
	}
	e.checkStateBased()
	if got := e.G.Obj(tok).Zone; got != state.ZCeased {
		t.Fatalf("warped token zone = %s, want ceased", got)
	}
}

// The control: a card in the same place is put onto the battlefield.
func TestADigReturnsAWarpedCard(t *testing.T) {
	t.Parallel()
	goblinSrc := "Name:Fixture Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	e, _, _ := newFixtureDeckWithTokens(t, 114, warpTopSrc)
	gob := onBoard(t, e, 0, goblinSrc)
	if o := e.G.Obj(gob); o == nil || o.IsToken || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: object %d is not a battlefield card (%+v)", gob, o)
	}
	warpOntoEmptyLibrary(t, e, gob)
	if toLibrary, back := warpDigMoves(e, gob); toLibrary != 1 || back != 1 {
		t.Fatalf("the card moved to the library %d times and back %d times, want 1 and 1", toLibrary, back)
	}
}
