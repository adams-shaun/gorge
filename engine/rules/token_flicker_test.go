package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 111.8: a token that has left the battlefield can't move to another zone
// or come back onto the battlefield. Splash Portal's shape is the one-
// resolution case: one ChangeZone exiles the target, then its DBReturn
// sub-ability moves the Remembered set from exile back to the battlefield.
// No state-based action runs between the two moves, so the return mover must
// carry the CR 111.8 guard itself; without it the token returns, its
// RememberChanged$ records it, and the conditional draw fires.
//
// The fixture is authored inline (never a corpus .txt, the licensing rule) and
// mirrors Splash Portal's exact A:/SVar: lines with Goblin in place of the
// corpus card's Bird/Frog/Otter/Rat type line, because the shared fixture
// token is r_1_1_goblin.
const flickerPortalSrc = "Name:Flicker Portal\nManaCost:1 U\nTypes:Instant\n" +
	"A:SP$ ChangeZone | ValidTgts$ Creature.YouCtrl | Origin$ Battlefield | Destination$ Exile | TgtPrompt$ Select target creature you control | RememberTargets$ True | SubAbility$ DBReturn\n" +
	"SVar:DBReturn:DB$ ChangeZone | Defined$ Remembered | Origin$ Exile | Destination$ Battlefield | RememberChanged$ True | ForgetOtherRemembered$ True | SubAbility$ DBDraw\n" +
	"SVar:DBDraw:DB$ Draw | Defined$ You | ConditionDefined$ Remembered | ConditionPresent$ Goblin | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:x\n"

// flickerTarget answers the fixture spell's target ask with want, failing if
// the ask (or the option) is not there -- the precondition the zone assertion
// below depends on.
func flickerTarget(t *testing.T, e *Engine, want state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the flicker spell's target ask, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == want {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("target %d not offered: %+v", want, d.Options)
}

// TestATokenFlickeredByChangeZoneDoesNotReturn pins the token half: the token
// is exiled (a token MAY leave the battlefield, CR 111.8) and then the return
// move is skipped, so it never comes back, no RememberChanged$ record is
// made, and the conditional draw does not happen. After the next state-based
// action the exiled token is a ZCeased tombstone.
func TestATokenFlickeredByChangeZoneDoesNotReturn(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeckWithTokens(t, 111, flickerPortalSrc)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	bf := e.G.Zone(state.ZBattlefield, 0)
	tok := bf[len(bf)-1]
	if !e.G.Obj(tok).IsToken || e.G.Obj(tok).Zone != state.ZBattlefield {
		t.Fatalf("precondition: object %d is not a battlefield token (%+v)", tok, e.G.Obj(tok))
	}

	addMana(t, e, 0, "CU")
	handBefore := len(e.G.Zone(state.ZHand, 0))
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	if handBefore < 1 || libBefore < 2 {
		t.Fatalf("precondition: hand=%d library=%d, want the spell and a library to draw from", handBefore, libBefore)
	}

	castFirst(t, e, "cast")
	flickerTarget(t, e, tok)
	passUntilStackEmpty(t, e, 20)
	// The cast itself took the spell out of hand and put it in the graveyard;
	// the spell resolves with no other card movement.
	if z := e.G.Obj(tok).Zone; z == state.ZBattlefield {
		t.Fatalf("flickered token zone = %s, want it exiled then ceased: CR 111.8 forbids the return", z)
	}
	e.checkStateBased()
	if got := e.G.Obj(tok).Zone; got != state.ZCeased {
		t.Fatalf("flickered token zone = %s, want ceased (it left the battlefield and cannot return)", got)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id == tok {
			t.Fatalf("battlefield = %v, still holds the flickered token", e.G.Zone(state.ZBattlefield, 0))
		}
	}
	// No draw: the spell's own card leaves hand (so handBefore-1), and the
	// library is untouched. Without the guard hand is handBefore (the token
	// returns and the draw replaces the spent card) and the library shrinks.
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore-1 {
		t.Fatalf("hand = %d, want %d: the goblin token must not return and the failed condition must not draw", got, handBefore-1)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore {
		t.Fatalf("library = %d, want %d: no card may be drawn when the Remembered return is skipped", got, libBefore)
	}
}

// TestANonTokenFlickeredByChangeZoneReturns is the control: the same spell on
// a real Goblin card. The guard is token-only, so the card leaves exile,
// re-enters the battlefield, is remembered again, and the conditional draw
// fires (CR 111.8 does not apply to cards).
func TestANonTokenFlickeredByChangeZoneReturns(t *testing.T) {
	t.Parallel()
	goblinSrc := "Name:Fixture Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	e, _, _ := newFixtureDeckWithTokens(t, 112, flickerPortalSrc)
	gob := onBoard(t, e, 0, goblinSrc)
	if o := e.G.Obj(gob); o == nil || o.IsToken || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: object %d is not a battlefield card (%+v)", gob, o)
	}

	addMana(t, e, 0, "CU")
	handBefore := len(e.G.Zone(state.ZHand, 0))
	libBefore := len(e.G.Zone(state.ZLibrary, 0))
	if handBefore < 1 || libBefore < 2 {
		t.Fatalf("precondition: hand=%d library=%d, want the spell and a library to draw from", handBefore, libBefore)
	}

	castFirst(t, e, "cast")
	flickerTarget(t, e, gob)
	passUntilStackEmpty(t, e, 20)

	if z := e.G.Obj(gob).Zone; z != state.ZBattlefield {
		t.Fatalf("flickered card zone = %s, want battlefield: a card returns from exile (CR 111.8 is token-only)", z)
	}
	// The spell leaves hand, the goblin returns, and the draw replaces the
	// spell card: hand is unchanged and the library shrank by one.
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
		t.Fatalf("hand = %d, want %d: the returned Goblin must make the conditional draw", got, handBefore)
	}
	if got := len(e.G.Zone(state.ZLibrary, 0)); got != libBefore-1 {
		t.Fatalf("library = %d, want %d: the Goblin's return must draw exactly one card", got, libBefore-1)
	}
}
