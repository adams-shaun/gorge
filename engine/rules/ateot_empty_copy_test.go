package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A sorcery that makes X token copies with a "sacrifice them at the beginning
// of the next end step" rider, written for this test.
const emptyCopyRiderSrc = "Name:Hollow Muster\nManaCost:X R\nTypes:Sorcery\n" +
	"A:SP$ CopyPermanent | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Pick a creature | NumCopies$ HollowCount | PumpKeywords$ Haste | AtEOT$ Sacrifice | SpellDescription$ Make X copies; they are sacrificed at the next end step.\n" +
	"SVar:HollowCount:Count$xPaid\n" +
	"Oracle:Create X tokens that are copies of target creature you control. Those tokens gain haste until end of turn. Sacrifice them at the beginning of the next end step.\n"

const emptyCopyBearSrc = "Name:Plain Hound\nManaCost:1\nTypes:Creature Dog\nPT:2/2\nOracle:\n"

// TestAtEOTRiderWithNoCopiesStillTriggers pins CR 603.7: the delayed
// triggered ability a CopyPermanent rider creates is made by the resolving
// spell whether or not any copy was made (X = 0), so it triggers at the next
// end step, goes on the stack and resolves doing nothing, as Forge and XMage
// put it there.
func TestAtEOTRiderWithNoCopiesStillTriggers(t *testing.T) {
	t.Parallel()
	e, cfg, find := etbConfig(t, 114, []string{emptyCopyRiderSrc, emptyCopyBearSrc}, nil)
	putCreature(t, e, 0, emptyCopyBearSrc)
	spell := find("Hollow Muster", 0)
	addMana(t, e, 0, "R")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 0)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		submitChoices(t, e, 0)
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("delayed registrations %d after an X=0 resolution, want 1", len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the delayed sacrifice never reached the stack when no copy was made")
	}
	if top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1]); top == nil || top.Ability == nil || top.Source != spell {
		t.Fatalf("stack top %+v, want the spell's delayed ability", top)
	}
	replayCheck(t, e, cfg)
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}
