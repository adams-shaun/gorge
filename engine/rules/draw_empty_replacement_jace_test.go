package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// jaceWielderSrc is a fixture in Jace, Wielder of Mysteries' shape: the same
// static Draw replacement, with its parameters reordered and its SVar renamed
// so it is not the corpus script (the loyalty abilities are not the subject).
const jaceWielderSrc = `Name:Jace, Wielder of Mysteries
ManaCost:1 U U U
Types:Legendary Planeswalker Fixture
Loyalty:4
R:Event$ Draw | ValidPlayer$ You | PresentZone$ Library | ActiveZones$ Battlefield | IsPresent$ Card.YouOwn | PresentCompare$ EQ0 | ReplaceWith$ WinNow | Description$ Fixture: win instead of drawing from an empty library.
SVar:WinNow:DB$ WinsGame | Defined$ You
`

// TestJaceWielderOfMysteriesWinsOnEmptyLibraryDraw pins CR 614.11 for Jace's
// line: an attempted draw from an empty library is replaced by the win, while
// a draw with a card left in the library is an ordinary draw.
func TestJaceWielderOfMysteriesWinsOnEmptyLibraryDraw(t *testing.T) {
	t.Parallel()
	e, cfg, herald, jace := cantLoseFixture(t, 8322, jaceWielderSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: herald, From: state.ZBattlefield, To: state.ZGraveyard})
	if jace == 0 || e.G.Obj(jace).Zone != state.ZBattlefield {
		t.Fatal("Jace is not on seat 1's battlefield")
	}
	cantLoseEmptyLibrary(t, e, 1)
	effects.DrawFor(e, 1)
	if e.G.Players[1].DrewFromEmpty || countDrawEmpty(e, 1, 0) != 0 {
		t.Fatal("the empty-library draw was marked instead of replaced")
	}
	e.checkStateBased()
	if !e.G.Over || e.G.Winner != 1 || e.G.Players[1].Lost {
		t.Fatalf("over=%v winner=%d lost=%v, want seat 1 to win", e.G.Over, e.G.Winner, e.G.Players[1].Lost)
	}
	replayCheck(t, e, cfg)

	e2, _, _, _ := cantLoseFixture(t, 8323, jaceWielderSrc)
	hand := len(e2.G.Zone(state.ZHand, 1))
	effects.DrawFor(e2, 1)
	if len(e2.G.Zone(state.ZHand, 1)) != hand+1 || e2.G.Over {
		t.Fatalf("non-empty draw: hand %d -> %d, over=%v", hand, len(e2.G.Zone(state.ZHand, 1)), e2.G.Over)
	}
}
