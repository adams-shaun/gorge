package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPotentialActionsReadRestrictedMana pins CR 106.6 on the potential walk:
// mana a source restricts to a class of payments counts toward a potential
// action only when the restriction admits that payment. Two Orbs of Dragonkind
// and a Mountain fund a Dragon's own {R} ability but not Loxodon Warhammer's
// Equip {3}, which only the Mountain's mana may pay.
func TestPotentialActionsReadRestrictedMana(t *testing.T) {
	testutil.CorpusDirOrSkip(t)
	e := layerEngine(t)
	e.G.Step = state.StepMain1
	e.G.Players[0].LandsPlayed = 1
	e.G.SetZone(state.ZHand, 0, nil)
	ready := func(src string) state.ObjID {
		id := onBoard(t, e, 0, src)
		e.G.Obj(id).SummonSick = false
		return id
	}
	ready("Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	ready(corpusCardText(t, "o/orb_of_dragonkind.txt"))
	ready(corpusCardText(t, "o/orb_of_dragonkind.txt"))
	hammer := ready(corpusCardText(t, "l/loxodon_warhammer.txt"))
	hatchling := ready(corpusCardText(t, "d/dragon_hatchling.txt"))

	equip, pump := false, false
	for _, a := range e.PotentialActions(0) {
		if a.Kind != "ability" {
			continue
		}
		switch a.Obj {
		case hammer:
			equip = true
		case hatchling:
			pump = true
		}
	}
	if equip {
		t.Errorf("Equip {3} is a potential action with one Mountain and two Orbs of Dragonkind, whose mana may not pay it")
	}
	if !pump {
		t.Errorf("the Dragon's {R} ability is not a potential action, though an Orb of Dragonkind funds it")
	}
}
