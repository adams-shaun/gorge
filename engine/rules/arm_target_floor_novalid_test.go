package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestArmTargetCostFloorNoValidTarget pins the e22395b47 branch: a
// target-reading Amount$ with NO ValidTarget$ whose gate passes at nil
// targets used to keep the printed cost as the floor (the gate said "not
// independently denied"), but a target-reading amount evaluated with no
// targets reads zero, so base does not already carry the reduction and the
// real per-target price is lower than that floor. Such a static must refuse
// the floor so the full per-target enumeration runs.
//
// The sibling fixture in arm_target_floor_test.go's a_count_amount_refuses
// carries a ValidTarget$, so it exercises the other branch and cannot see
// this one.
func TestArmTargetCostFloorNoValidTarget(t *testing.T) {
	t.Parallel()
	printed := cost.Cost{Generic: 4}
	printed.Colored[state.MW] = 1

	e, _, _ := newFixtureDeck(t, 9421, ppFiller)
	county := card(t, "Name:Arm Floor Target Count Spell\nManaCost:4 W\nTypes:Instant\n"+
		"S:Mode$ ReduceCost | Type$ Spell | ValidCard$ Card.Self | Amount$ Count$Valid Creature.targeted | EffectZone$ All | Description:x\n"+
		"A:SP$ Destroy | SpellDescription$ Destroy target creature. | ValidTgts$ Creature\nOracle:x\n")
	spell := handCard(e, county, 0)
	statics := e.collectCostStatics()

	// Precondition: the collected static is the branch's shape -- the cast's
	// own, no ValidTarget$, a target-reading amount -- and its gate passes
	// at nil targets (ok) without a target-independent denial. Without that
	// verdict the assertion below would pass for the wrong reason.
	found := false
	for _, sv := range statics.reduce {
		if sv.Source != spell || sv.ParamStr(cards.PKValidTarget) != "" || !costStaticReadsTargets(sv) {
			continue
		}
		found = true
		if ok, indepFail := e.costStaticGate(sv, "ReduceCost", 0, spell, spellScope(""), nil, false); !ok || indepFail {
			t.Fatalf("precondition: gate (ok=%v indepFail=%v), want a passing target-independent gate", ok, indepFail)
		}
	}
	if !found {
		t.Fatal("precondition: the target-reading, no-ValidTarget$ reduction was not collected")
	}
	if _, ok := e.planTargetCostFloor(statics, 0, spell, spellScope(""), printed); ok {
		t.Fatal("a no-ValidTarget$ target-reading amount's floor is no subtraction")
	}
}
