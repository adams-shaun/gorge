package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const laboratoryManiacSrc = `Name:Laboratory Maniac
ManaCost:2 U
Types:Creature Human Wizard
PT:2/2
R:Event$ Draw | ValidPlayer$ You | ActiveZones$ Battlefield | ReplaceWith$ WinNow | PresentZone$ Library | IsPresent$ Card.YouOwn | PresentCompare$ EQ0 | Description$ Fixture: win instead of drawing from an empty library.
SVar:WinNow:DB$ WinsGame | Defined$ You
`

// TestLaboratoryManiacWinsOnEmptyLibraryDraw pins CR 614.11: a Draw
// replacement is applied to an attempted draw from an empty library, so
// Laboratory Maniac's controller wins instead of losing at the next
// state-based check. The same draw with a card left in the library is an
// ordinary draw, and without Laboratory Maniac the empty draw still loses.
func TestLaboratoryManiacWinsOnEmptyLibraryDraw(t *testing.T) {
	t.Parallel()
	e, cfg, herald, maniac := cantLoseFixture(t, 8320, laboratoryManiacSrc)
	// The fixture's Herald of Eternal Dawn (seat 0) stops seat 1 winning; the
	// win under test is not its subject, so it leaves the battlefield.
	e.emit(events.Event{Kind: events.MoveZone, Obj: herald, From: state.ZBattlefield, To: state.ZGraveyard})
	if maniac == 0 || e.G.Obj(maniac).Zone != state.ZBattlefield {
		t.Fatal("Laboratory Maniac is not on seat 1's battlefield")
	}
	cantLoseEmptyLibrary(t, e, 1)
	if n := len(e.G.Zone(state.ZLibrary, 1)); n != 0 {
		t.Fatalf("seat 1 library holds %d cards, want 0", n)
	}

	effects.DrawFor(e, 1)
	if e.G.Players[1].DrewFromEmpty || countDrawEmpty(e, 1, 0) != 0 {
		t.Fatal("the empty-library draw was marked instead of replaced")
	}
	e.checkStateBased()
	if e.G.Players[1].Lost {
		t.Fatal("seat 1 lost with Laboratory Maniac on the battlefield")
	}
	if !e.G.Over || e.G.Winner != 1 {
		t.Fatalf("game over=%v winner=%d, want seat 1 to win", e.G.Over, e.G.Winner)
	}
	replayCheck(t, e, cfg)

	// Control: a library with a card in it is an ordinary draw, no win.
	e2, _, _, _ := cantLoseFixture(t, 8321, laboratoryManiacSrc)
	hand := len(e2.G.Zone(state.ZHand, 1))
	effects.DrawFor(e2, 1)
	if len(e2.G.Zone(state.ZHand, 1)) != hand+1 || e2.G.Over {
		t.Fatalf("non-empty draw: hand %d -> %d, over=%v", hand, len(e2.G.Zone(state.ZHand, 1)), e2.G.Over)
	}
}
