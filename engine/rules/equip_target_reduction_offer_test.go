package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"

	"github.com/adams-shaun/gorge/engine/state"
)

// A ReduceCost static that discounts equip abilities targeting its own
// creature ("Equip abilities you activate that target this creature cost {2}
// less") makes an Equip {7} payable from five mana: the cost is locked in after
// the target is chosen (CR 601.2f via 602.2b), so the offer must price the
// equip against the reducing creature as a potential target.
const etrChampionSrc = "Name:Test Champion\nManaCost:3 R W\nTypes:Legendary Creature Human Soldier\nPT:4/4\n" +
	"S:Mode$ ReduceCost | ValidTarget$ Card.Self | ValidSpell$ Activated.Equip | Activator$ You | Amount$ 2\n" +
	"Oracle:Equip abilities you activate that target this creature cost {2} less to activate.\n"

const etrHeavyEquipSrc = "Name:Test Heavy Weapon\nManaCost:7\nTypes:Legendary Artifact Equipment\n" +
	"S:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddPower$ 7 | AddToughness$ 7\n" +
	"K:Equip:7\nOracle:Equip {7}\n"

func etrOffered(e *Engine, p state.PlayerID, obj state.ObjID) bool {
	for _, o := range e.legalActions(p) {
		if o.Kind == "ability" && o.Obj == obj {
			return true
		}
	}
	return false
}

func TestEquipOfferedWhenATargetDependentReductionMakesItPayable(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	cgPlace(t, e, 0, card(t, etrChampionSrc))
	cgPlace(t, e, 0, card(t, cgBear))
	weapon := cgPlace(t, e, 0, card(t, etrHeavyEquipSrc))
	e.G.Players[0].Pool[state.MC] = 4
	if etrOffered(e, 0, weapon) {
		t.Fatalf("equip {7} offered with only {4}: even the {2} discount leaves it unpayable")
	}
	e.G.Players[0].Pool[state.MC] = 5
	if !etrOffered(e, 0, weapon) {
		t.Fatalf("equip {7} not offered with {5}: targeting the reducing creature costs {5}")
	}
}

func TestEquipOfferedFromLandsWhenATargetDependentReductionMakesItPayable(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	cgPlace(t, e, 0, card(t, etrChampionSrc))
	cgPlace(t, e, 0, card(t, cgBear))
	weapon := cgPlace(t, e, 0, card(t, etrHeavyEquipSrc))
	putLands(t, e, 0, 4)
	if etrOffered(e, 0, weapon) {
		t.Fatalf("equip {7} offered with four lands: even the {2} discount leaves it unpayable")
	}
	putLands(t, e, 0, 1)
	if !etrOffered(e, 0, weapon) {
		t.Fatalf("equip {7} not offered with five lands: targeting the reducing creature costs {5}")
	}
}

// The offered activation pays: targeting the reducing creature, the armed
// witness taps exactly the five lands and the Equipment attaches.
func TestEquipTargetDependentReductionActivationPaysFromLands(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	champ := cgPlace(t, e, 0, card(t, etrChampionSrc))
	cgPlace(t, e, 0, card(t, cgBear))
	weapon := cgPlace(t, e, 0, card(t, etrHeavyEquipSrc))
	lands := putLands(t, e, 0, 5)
	activateAbilityOn(t, e, weapon)
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind == decision.KTarget {
			answerKTarget(t, e, champ)
			continue
		}
		if d.Kind == decision.KPriority && len(e.G.Stack) == 0 {
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %s: %+v", d.Kind, d.Options)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		submitChoices(t, e, pass)
	}
	if got := e.G.Obj(weapon).AttachedTo; got != champ {
		t.Fatalf("weapon attached to %d, want the champion %d", got, champ)
	}
	for _, l := range lands {
		if !e.G.Obj(l).Tapped {
			t.Fatalf("land %d untapped: equip {7} less {2} needs all five", l)
		}
	}
}
