package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const maulerSrc = "Name:Dwarven Mauler\nManaCost:R\nTypes:Creature Dwarf Warrior\nPT:2/1\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2\n" +
	"Oracle:Equip abilities you activate that target this creature cost {2} less to activate.\n"

const stingEquipSrc = "Name:Sting Probe\nManaCost:2\nTypes:Legendary Artifact Equipment\nK:Equip:3\nOracle:Equip {3}\n"

// A target-gated equip reduction prices the offer at the cheapest target.
func TestEquipTargetGatedReduceOffered(t *testing.T) {
	e, _, _ := newFixtureDeck(t, 71, stingEquipSrc, maulerSrc)
	putToken(t, e, 0, stingEquipSrc, state.ZBattlefield)
	putCreature(t, e, 0, maulerSrc)
	putLands(t, e, 0, 2)
	if kinds(e.legalActions(0))["ability"] != 1 {
		t.Fatalf("equip should be offered at {1}: %v", kinds(e.legalActions(0)))
	}
}

// Without the Mauler two lands cannot pay {3}: the offer is the discount's.
func TestEquipTargetGatedReduceNeedsMauler(t *testing.T) {
	e, _, _ := newFixtureDeck(t, 72, stingEquipSrc, maulerSrc, testBearSrc)
	putToken(t, e, 0, stingEquipSrc, state.ZBattlefield)
	putCreature(t, e, 0, testBearSrc)
	putLands(t, e, 0, 2)
	if kinds(e.legalActions(0))["ability"] != 0 {
		t.Fatal("equip {3} must not be offered on two lands")
	}
}
