package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestXBoundTargetOfferKeepsFilterFeeSource pins CR 605.3b and 601.2g: an {X}{B}
// spell whose target needs X = 1 is offered when the black can only come from a
// filter ("{1}, {T}: Add one mana of any color", Unknown Shores) whose fee a
// plains pays. The payment planner's census does not cover that fee, so its
// "insufficient" verdict is no proof of an unpayable announcement.
func TestXBoundTargetOfferKeepsFilterFeeSource(t *testing.T) {
	t.Parallel()
	const glare = "Name:Killing Glare\nManaCost:X B\nTypes:Instant\n" +
		"A:SP$ Destroy | ValidTgts$ Creature.powerLEX | TgtPrompt$ Pick a creature | SpellDescription$ Destroy a small one.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const bear = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"
	const plains = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"
	const shores = "Name:Unknown Shores\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Produced$ C | Cost$ T | SpellDescription$ Make one colorless.\n" +
		"A:AB$ Mana | Produced$ Any | Cost$ 1 T | SpellDescription$ Make one mana of a color.\nOracle:x\n"
	offered := func(t *testing.T, lands ...string) bool {
		e := handEngine(t, card(t, glare))
		for _, l := range lands {
			onBoardGrant(t, e, 0, l)
		}
		onBoard(t, e, 1, bear)
		e.priorityRound()
		// Lands are tapped for mana one by one, so a cast they alone pay is a
		// potential play, priced by the planner, not a priority option.
		id := e.G.Zone(state.ZHand, 0)[0]
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" && pp.Action.Obj == id {
				return pp.Reason == ""
			}
		}
		return false
	}
	if offered(t, plains, plains) {
		t.Fatal("precondition: offered with no black source at all")
	}
	if !offered(t, plains, plains, plains, shores) {
		t.Fatal("not offered with X=1 payable as plains for the filter's fee, shores for the {B}, plains for the {X}")
	}
}
