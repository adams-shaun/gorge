package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestXBoundTargetOfferWithholdsUnpayableSignet pins CR 601.2g-h and 605.3b: a
// Signet's "{1},{T}: Add {B}{G}" spends mana to make mana, so the one Island
// beside it pays either the Signet's {1} or the {U} of an {X}{U} spell, never
// both, and the Signet's own output cannot pay its fee. Repeal's only target
// has mana value 1, so the cast costs {1}{U}; Island + Signet cannot pay it,
// while Island + Island can.
func TestXBoundTargetOfferWithholdsUnpayableSignet(t *testing.T) {
	t.Parallel()
	const repeal = "Name:Repeal\nManaCost:X U\nTypes:Instant\n" +
		"A:SP$ ChangeZone | ValidTgts$ Artifact.OppCtrl+cmcEQX | TgtPrompt$ Pick an artifact | Origin$ Battlefield | Destination$ Hand | SpellDescription$ Return it.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const opt = "Name:Opt\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ Dig | DigNum$ 1 | ChangeNum$ 1 | Optional$ True | SpellDescription$ Look.\nOracle:x\n"
	const rock = "Name:Rock\nManaCost:1\nTypes:Artifact\nOracle:x\n"
	const island = "Name:Island\nTypes:Basic Land Island\nOracle:x\n"
	const signet = "Name:Golgari Signet\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Mana | Produced$ B G | Cost$ 1 T | SpellDescription$ Make one black and one green.\nOracle:x\n"
	offered := func(t *testing.T, spell string, board ...string) bool {
		e := handEngine(t, card(t, spell))
		for _, c := range board {
			onBoardGrant(t, e, 0, c)
		}
		onBoard(t, e, 1, rock)
		e.priorityRound()
		id := e.G.Zone(state.ZHand, 0)[0]
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" && pp.Action.Obj == id {
				return pp.Reason == ""
			}
		}
		return false
	}
	if !offered(t, repeal, island, island) {
		t.Fatal("precondition: not offered with two Islands ({1}{U} payable)")
	}
	if offered(t, repeal, island) {
		t.Fatal("precondition: offered with one Island")
	}
	if !offered(t, opt, island, signet) {
		t.Fatal("precondition: plain {U} spell not offered with Island + Signet")
	}
	if offered(t, repeal, island, signet) {
		t.Fatal("Repeal offered with Island + Signet: the Signet's output pays its own fee")
	}
}
