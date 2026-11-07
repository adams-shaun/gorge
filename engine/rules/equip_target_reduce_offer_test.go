package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Paraphrased fixtures: an Equip {3} artifact, and a creature whose static
// makes equip abilities that target it cost {2} less (CR 601.2f prices the
// activation after its target is chosen).
const targetReducedEquipSrc = "Name:Probe Staff\nManaCost:1 W\nTypes:Artifact Equipment\nK:Equip:3\n" +
	"Oracle:Equip {3}\n"

const equipDiscountTargetSrc = "Name:Probe Champion\nManaCost:3 R W\nTypes:Legendary Creature Human Soldier\nPT:4/4\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2\n" +
	"Oracle:Equip abilities you activate that target this creature cost {2} less to activate.\n"

const probeBearSrc = "Name:Probe Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:\n"

const probeLandSrc = "Name:Probe Land\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ Combo G U | SpellDescription$ Probe land mana.\nOracle:\n"

// TestEquipOfferedAtTargetReducedCostFromUntappedLand pins the autopay
// offer: with an empty pool and one untapped land, the equip is payable only
// by targeting the discounting creature ({1}), so the planner must price it
// per target as the offer gate does, whether or not the Equipment is already
// attached elsewhere.
func TestEquipOfferedAtTargetReducedCostFromUntappedLand(t *testing.T) {
	for _, attached := range []bool{false, true} {
		e, _, _ := newFixtureDeck(t, 71, targetReducedEquipSrc, equipDiscountTargetSrc, probeBearSrc, probeLandSrc)
		staff := putCreature(t, e, 0, targetReducedEquipSrc)
		bear := putCreature(t, e, 0, probeBearSrc)
		putCreature(t, e, 0, equipDiscountTargetSrc)
		land := putCreature(t, e, 0, probeLandSrc)
		if attached {
			attachFixture(t, e, staff, bear)
		}
		toMain1(t, e)
		if got := e.G.Players[0].Pool.Total(); got != 0 {
			t.Fatalf("attached=%v: pool holds %d mana, want empty (the land must pay)", attached, got)
		}
		if lo := e.G.Obj(land); lo == nil || lo.Zone != state.ZBattlefield || lo.Tapped {
			t.Fatalf("attached=%v: the land is not an untapped battlefield source", attached)
		}
		if got := e.AbilityCosts(0, staff); len(got) != 1 || got[0] != "3" {
			t.Fatalf("attached=%v: printed equip cost %v, want [3] (unpayable from one land)", attached, got)
		}
		n := 0
		for _, o := range e.legalActions(0) {
			if o.Kind == "ability" && o.Obj == staff {
				n++
			}
		}
		if n != 1 {
			t.Errorf("attached=%v: equip onto the champion costs {1} and one land pays it; got %d equip options", attached, n)
		}
	}
}
