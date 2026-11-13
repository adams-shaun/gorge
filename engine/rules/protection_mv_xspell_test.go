package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	protMVTorch = "Name:Test Torch\nManaCost:X R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X\nSVar:X:Count$xPaid\nOracle:x\n"
	protMVSkulk = "Name:Test Skulk\nManaCost:2\nTypes:Creature Human\nPT:3/3\nK:Protection:Card.cmcGE3:mana value 3 or greater\nOracle:x\n"
)

// protMVTargetsAt casts Test Torch at announced X and reports whether the
// protected Test Skulk is among the offered targets (CR 601.2b announces X
// before 601.2c chooses targets, so the spell's mana value is 1+X).
func protMVTargetsAt(t *testing.T, x int) (offered bool, skulk state.ObjID) {
	t.Helper()
	e := handEngine(t, card(t, protMVTorch))
	skulk = battlefieldFixture(t, e, 1, protMVSkulk)
	bear := battlefieldFixture(t, e, 1, costredBear)
	e.G.Players[0].Pool[state.MR] = 4
	spell := e.G.Zone(state.ZHand, 0)[0]
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatal("Test Torch not offered")
	}
	castMode(t, e, spell, "")
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x" {
			submitChoices(t, e, x)
			continue
		}
		if d.Kind == decision.KTarget {
			sawBear := false
			for _, o := range d.Options {
				if o.Obj == skulk {
					offered = true
				}
				if o.Obj == bear {
					sawBear = true
				}
			}
			if !sawBear {
				t.Fatalf("precondition: the unprotected bear is not offered: %+v", d.Options)
			}
			return offered, skulk
		}
		submitChoices(t, e, 0)
	}
	t.Fatalf("no target ask reached; pending %+v", e.Pending())
	return
}

// TestProtectionManaValueXSpellTargetOffer: an X spell whose announced X
// makes its mana value 3 is not offered a "protection from mana value 3 or
// greater" creature as a target, and at X=1 (mana value 2) it is.
func TestProtectionManaValueXSpellTargetOffer(t *testing.T) {
	t.Parallel()
	if offered, _ := protMVTargetsAt(t, 1); !offered {
		t.Fatal("X=1 (mana value 2): the Skulk should be a legal target")
	}
	if offered, _ := protMVTargetsAt(t, 2); offered {
		t.Fatal("X=2 (mana value 3): the Skulk was offered as a target of a mana value 3 spell")
	}
}
