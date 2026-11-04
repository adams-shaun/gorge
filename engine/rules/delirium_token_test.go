package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A token that has just reached the graveyard waits there for state-based
// actions (CR 704.5d) but is not a card (CR 111.7): the card types among the
// cards in a graveyard, and the cards counted for Threshold, leave it out.
func TestGraveyardCensusSkipsATokenWaitingToCease(t *testing.T) {
	e, _, _ := gateFixture(t, 951, "Descend upon the Sinful",
		deliriumBearSrc, deliriumPlainsSrc, deliriumBoltSrc, deliriumOathSrc)
	millExtras(t, e, "Delirium Bear", "Delirium Plains", "Delirium Bolt", "Delirium Oath")
	if !e.DeliriumHolds(0) {
		t.Fatal("four card types in the graveyard: delirium should hold")
	}
	var oath *state.Object
	for _, id := range e.G.Zone(state.ZGraveyard, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().Name == "Delirium Oath" {
			oath = o
		}
	}
	if oath == nil {
		t.Fatal("no Delirium Oath in the graveyard")
	}
	oath.IsToken = true
	if e.DeliriumHolds(0) {
		t.Fatal("the fourth type belongs to a token in the graveyard: delirium should not hold")
	}
	if got := e.graveyardCardTypeCount(0); got != 3 {
		t.Fatalf("graveyard card types = %d, want 3 (the token is no card)", got)
	}
}

// "As long as there are two or more creature cards in your graveyard" counts
// cards only: a token that has just died waits in the graveyard for
// state-based actions (CR 704.5d) and is no card (CR 111.7), so it must not
// switch Killmonger's bonus on, and it does once the second card is real.
func TestGraveyardPresenceGateSkipsATokenWaitingToCease(t *testing.T) {
	e, _, _ := gateFixture(t, 952, "Killmonger, Scourge of Wakanda", deliriumBearSrc, deliriumBearSrc)
	kill := gateMoveFromLibrary(t, e, "Killmonger, Scourge of Wakanda", state.ZBattlefield)
	_ = gateMoveFromLibrary(t, e, "Delirium Bear", state.ZGraveyard)
	if got := e.Power(kill); got != 3 {
		t.Fatalf("one creature card in the graveyard: power = %d, want 3", got)
	}
	second := findInZones(t, e, 0, "Delirium Bear")
	e.G.Obj(second).IsToken = true // a creature token that has just died
	e.emit(events.Event{Kind: events.MoveZone, Obj: second, From: state.ZLibrary, To: state.ZGraveyard})
	if got := e.Power(kill); got != 3 {
		t.Fatalf("one creature card and a token waiting to cease: power = %d, want 3", got)
	}
}
