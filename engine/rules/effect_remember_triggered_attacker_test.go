package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEffectRememberTriggeredAttackerUnblockable plays Gravelgill Scoundrel's
// attack trigger end to end: after the optional tap cost is paid, the Effect
// remembers the triggering attacker (RememberObjects$ TriggeredAttacker) and
// the registered CantBlockBy restriction stops the defender's creature from
// blocking it, while an attacker the effect did not remember stays blockable.
func TestEffectRememberTriggeredAttackerUnblockable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "Gravelgill Scoundrel")
	scoundrel := searchMoveByName(t, e, "Gravelgill Scoundrel", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	blocker := onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature Construct\nPT:1/1\nOracle:x\n")
	if scoundrel == bear {
		t.Fatal("fixture degenerate: scoundrel and tap-cost creature are the same object")
	}
	e.G.Obj(scoundrel).SummonSick = false
	e.G.Obj(bear).SummonSick = false
	if n := effectCantBlockByCount(e); n != 0 {
		t.Fatalf("precondition: %d CantBlockBy effects before the attack", n)
	}

	e.pending = nil
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, scoundrel)
	// Take the optional trigger, then pay its tap cost with the other
	// creature.
	paid := false
	for i := 0; i < 8 && !paid; i++ {
		d := passUntilNonPriority(t, e, 40)
		switch {
		case d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_tap":
			if d.Options[0].Obj != bear {
				t.Fatalf("tap election offers %d, want the other creature %d", d.Options[0].Obj, bear)
			}
			submitChoices(t, e, d.Options[0].Index)
			paid = true
		case d.Kind == decision.KTriggerOptional:
			submitChoices(t, e, 0)
		default:
			t.Fatalf("unexpected decision before the tap election: %+v", d)
		}
	}
	if !paid {
		t.Fatal("the tap election was never offered")
	}
	passUntilStackEmpty(t, e, 40)
	if n := effectCantBlockByCount(e); n != 1 {
		t.Fatalf("CantBlockBy effects after the trigger resolved = %d, want 1", n)
	}

	e.G.Obj(scoundrel).IsAttacking, e.G.Obj(scoundrel).Attacking = true, 1
	e.G.Obj(bear).IsAttacking, e.G.Obj(bear).Attacking = true, 1
	if e.canBlock(blocker, scoundrel) {
		t.Error("the Scoundrel was blockable after its trigger remembered it")
	}
	if !e.canBlock(blocker, bear) {
		t.Error("an attacker outside the remembered set was also restricted (over-broad)")
	}
}
