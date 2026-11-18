package rules

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlainCastOfferJudgesTargetsWithTheCardOnTheStack: CR 601.2a moves the
// card to the stack before CR 601.2c asks for targets, so the plain cast offer
// in the priority walk must not offer a spell whose only target is a creature
// whose power is the cards in its controller's hand (Body of Knowledge) and
// shrinks below the spell's threshold once the spell leaves the hand.
// lockstep tail-57 pair 62: Vanquish the Foul.
func TestPlainCastOfferJudgesTargetsWithTheCardOnTheStack(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9631, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	for i := 0; i < 2; i++ {
		onBoard(t, e, 0, "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n")
	}
	onBoard(t, e, 0, "Name:Hand Glory\nManaCost:W\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ Creature.OppCtrl | AddPower$ X | Description$ x\n"+
		"SVar:X:Count$ValidHand Card.YouOwn\nOracle:x\n")
	onBoard(t, e, 1, "Name:Target Dummy\nManaCost:1\nTypes:Creature Test\nPT:0/1\nOracle:x\n")
	n := len(e.G.Zone(state.ZHand, 0)) + 2
	bolt := func(name string, ge int) state.ObjID {
		return putInHand(t, e, 0, card(t, fmt.Sprintf("Name:%s\nManaCost:W\nTypes:Instant\n"+
			"A:SP$ Destroy | ValidTgts$ Creature.powerGE%d | TgtPrompt$ x\nOracle:x\n", name, ge)))
	}
	vanishing := bolt("Vanishing Bolt", n)
	steady := bolt("Steady Bolt", n-1)
	toMain1(t, e)
	addMana(t, e, 0, "WW")
	if !e.castTargetsAvailable(0, vanishing, e.G.Obj(vanishing).Face().SpellAbility()) {
		t.Fatal("precondition: the in-hand census must admit the vanishing bolt")
	}
	cast := func(id state.ObjID) int {
		c := 0
		for _, o := range e.legalActions(0) {
			if o.Obj == id && o.Kind == "cast" {
				c++
			}
		}
		return c
	}
	if got := cast(steady); got != 1 {
		t.Errorf("steady bolt cast offers = %d, want 1", got)
	}
	if got := cast(vanishing); got != 0 {
		t.Errorf("vanishing bolt cast offers = %d, want 0: its target is gone once it leaves the hand", got)
	}
}
