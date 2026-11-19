package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const xBoundCharmSrc = "Name:Past Confronter\nManaCost:X B\nTypes:Sorcery\n" +
	"A:SP$ Charm | Announce$ X | Choices$ Back,Drain\n" +
	"SVar:Back:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Planeswalker.YouOwn+cmcLEX\n" +
	"SVar:Drain:DB$ RemoveCounter | CounterType$ LOYALTY | CounterNum$ Y | ValidTgts$ Planeswalker.OppCtrl\n" +
	"SVar:X:Count$xPaid\nSVar:Y:SVar$X/Times.2\nOracle:x\n"

const xBoundWalkerSrc = "Name:Past Walker\nManaCost:3 B B\nTypes:Legendary Planeswalker Walker\nLoyalty:5\nOracle:x\n"

// TestCharmXBoundModeTargetNeedsPayableX: a Choices$ charm whose mode target is
// bounded by the announced {X} (Confront the Past: a planeswalker card with
// mana value X or less) is offered only if some payable X leaves a mode a
// target (CR 601.2b, 601.2h). With one black mana the only X is 0, which names
// no mana value 5 planeswalker; with six mana X = 5 does.
func TestCharmXBoundModeTargetNeedsPayableX(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mana string
		want bool
	}{{"B", false}, {"BBBBBB", true}} {
		e, _, id := newFixtureDeck(t, 6015, xBoundCharmSrc)
		inZoneCard(t, e, 0, state.ZGraveyard, xBoundWalkerSrc)
		addMana(t, e, 0, tc.mana)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
			t.Fatalf("precondition: the charm is not in hand: %+v", o)
		}
		if got := castOffered(e, id); got != tc.want {
			t.Fatalf("mana %q: cast offered = %v, want %v", tc.mana, got, tc.want)
		}
	}
}
