package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// The consumption-only arm's floor gate (planTargetCostFloor) prices the
// cheapest one-target assignment without the per-target enumeration: the
// Rebuke-shaped cast's {3}-less tapped-creature reduction is a literal
// generic amount, so the floor is the printed cost less it, and any cost
// static the floor cannot price a subtraction of (a count amount, a
// target-dependent SetCost) refuses the floor instead. The assertions pin
// the floor's own answers; the enumeration it skips is the arm's contract
// (the consumption-only test's verdicts).
func TestArmTargetCostFloor(t *testing.T) {
	t.Parallel()
	printed := cost.Cost{Generic: 4}
	printed.Colored[state.MW] = 1
	want := cost.Cost{Generic: 1}
	want.Colored[state.MW] = 1

	t.Run("literal_target_reduction_is_the_floor", func(t *testing.T) {
		e, _, _ := newFixtureDeck(t, 9417, ppFiller)
		spell := handCard(e, card(t, acoSpell), 0)
		floor, ok := e.planTargetCostFloor(e.collectCostStatics(), 0, spell, spellScope(""), printed)
		if !ok || floor.Generic != want.Generic || floor.Colored != want.Colored ||
			len(floor.Hybrid) != 0 || len(floor.Sac) != 0 {
			t.Fatalf("floor (%v, %v), want {1}{W} under the {3} tapped-creature reduction", floor, ok)
		}
	})
	t.Run("another_casts_self_static_is_skipped", func(t *testing.T) {
		e, _, _ := newFixtureDeck(t, 9418, ppFiller)
		spell := handCard(e, card(t, acoSpell), 0)
		other := card(t, acoSpell)
		other.Faces[0].Name = "Arm Consumption Spell Two"
		handCard(e, other, 0)
		floor, ok := e.planTargetCostFloor(e.collectCostStatics(), 0, spell, spellScope(""), printed)
		if !ok || floor.Generic != want.Generic || floor.Colored != want.Colored {
			t.Fatalf("floor (%v, %v), want {1}{W}: the other cast's Card.Self static never prices this one", floor, ok)
		}
	})
	t.Run("a_count_amount_refuses_the_floor", func(t *testing.T) {
		e, _, _ := newFixtureDeck(t, 9419, ppFiller)
		county := card(t, "Name:Arm Floor Count Spell\nManaCost:4 W\nTypes:Instant\n"+
			"S:Mode$ ReduceCost | Type$ Spell | ValidCard$ Card.Self | Amount$ Count$InYourHand | EffectZone$ All | ValidTarget$ Creature.tapped | Description:x\n"+
			"A:SP$ Destroy | SpellDescription$ Destroy target creature. | ValidTgts$ Creature\nOracle:x\n")
		spell := handCard(e, county, 0)
		if _, ok := e.planTargetCostFloor(e.collectCostStatics(), 0, spell, spellScope(""), printed); ok {
			t.Fatal("a count amount's floor is no subtraction")
		}
	})
	t.Run("a_target_setcost_refuses_the_floor", func(t *testing.T) {
		e, _, _ := newFixtureDeck(t, 9420, ppFiller)
		setter := card(t, "Name:Arm Floor Set Spell\nManaCost:4 W\nTypes:Instant\n"+
			"S:Mode$ SetCost | Type$ Spell | ValidCard$ Card.Self | Amount$ 2 | EffectZone$ All | ValidTarget$ Creature.tapped | Description:x\n"+
			"A:SP$ Destroy | SpellDescription$ Destroy target creature. | ValidTgts$ Creature\nOracle:x\n")
		spell := handCard(e, setter, 0)
		if _, ok := e.planTargetCostFloor(e.collectCostStatics(), 0, spell, spellScope(""), printed); ok {
			t.Fatal("a target-dependent SetCost's floor is not a subtraction of the printed cost")
		}
	})
}
