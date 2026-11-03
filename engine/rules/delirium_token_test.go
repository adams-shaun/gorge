package rules

import (
	"testing"

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
