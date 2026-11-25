package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Cut a Deal: "Each opponent draws a card, then you draw a card for each
// opponent who drew a card this way." (Draw Defined$ Opponent RememberDrawn$
// AllReplaced; X = PlayerCountOpponents$HasPropertyIsRememberedOrController:
// gorge counts the opponents controlling a drawn card; kernel ExOppExtreme
// OppExtRemCtrl). Four-seat games play in lockstep; the mutation
// (DebugRemCtrlNone) draws the caster nothing.
var cutADealDeck = []cardset.Entry{
	{Name: "Cut a Deal", Count: 10}, {Name: "Savannah Lions", Count: 6},
	{Name: "Plains", Count: 24},
}

func TestFourSeatCutADealLockstep(t *testing.T) {
	atkShapeLockstep(t, cutADealDeck, 12, 0, 0, nil, nil, &dirac.DebugRemCtrlNone)
}
