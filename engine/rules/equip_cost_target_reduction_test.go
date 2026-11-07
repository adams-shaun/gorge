package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A cost-reduction static that reads the chosen target (Forge's
// `ValidTarget$ Card.Self`, Cloud, Planet's Champion's "Equip abilities you
// activate that target Cloud cost {2} less") must price the activation at
// the offer: an Equip {3} targeting Cloud is offered from one untapped
// Mountain, exactly as dirac offers it. The offer gate's potential-target
// retry is what makes this payable (CR 601.2f).

const equipTargetReduceBladeSrc = "Name:Offer Discount Blade\nManaCost:1\nTypes:Artifact Equipment\nK:Equip:3\nOracle:x\n"

const equipTargetReduceCloudSrc = "Name:Offer Discount Cloud\nManaCost:3 R W\nTypes:Legendary Creature Human Soldier\nPT:4/4\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2 | Description$ x\nOracle:x\n"

const equipTargetReduceBearSrc = "Name:Offer Discount Bear\nManaCost:2 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

const equipTargetReduceMountainSrc = "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"

// equipOffered reports whether the "ability" option for the blade's Equip is
// in the pending priority decision.
func equipOffered(e *Engine, blade state.ObjID) bool {
	_, ok := findAbilityOption(e, blade, 0)
	return ok
}

// TestEquipCostReducedByTargetStaticOffersTheActivation: with Cloud as a
// legal Equip target, one untapped Mountain and an empty pool, the {3} Equip
// is offered because the {2} target-static reduction makes it {1}.
func TestEquipCostReducedByTargetStaticOffersTheActivation(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 424242, equipTargetReduceBladeSrc, equipTargetReduceCloudSrc)
	moveSeeded(t, e, 0, equipTargetReduceCloudSrc, state.ZBattlefield)
	blade := moveSeeded(t, e, 0, equipTargetReduceBladeSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, equipTargetReduceMountainSrc, state.ZBattlefield)
	e.priorityRound()
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: the pool holds %d mana; the offer must be priced from the untapped Mountain", e.G.Players[0].Pool.Total())
	}
	if !equipOffered(e, blade) {
		t.Fatalf("Equip {3} targeting Cloud is not offered from one untapped Mountain under the {2} target-static reduction: %+v", e.Pending().Options)
	}
}

// TestEquipCostReducedByTargetStaticNeedsTheTarget: the same board with a
// vanilla creature in place of Cloud keeps the printed {3}, so the one
// Mountain cannot pay it and the Equip is withheld. This is the precondition
// the positive test's offer depends on: the reduction, not a free activation.
func TestEquipCostReducedByTargetStaticNeedsTheTarget(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 424243, equipTargetReduceBladeSrc, equipTargetReduceBearSrc)
	moveSeeded(t, e, 0, equipTargetReduceBearSrc, state.ZBattlefield)
	blade := moveSeeded(t, e, 0, equipTargetReduceBladeSrc, state.ZBattlefield)
	moveSeeded(t, e, 0, equipTargetReduceMountainSrc, state.ZBattlefield)
	e.priorityRound()
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: the pool holds %d mana", e.G.Players[0].Pool.Total())
	}
	if equipOffered(e, blade) {
		t.Fatalf("Equip {3} is offered from one Mountain with no target-static reduction: %+v", e.Pending().Options)
	}
}
