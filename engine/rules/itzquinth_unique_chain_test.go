package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTriggerUniqueChainNeedsTwoCreatures pins CR 603.3d on a triggered
// ability's chain: its root "target Dinosaur you control" and its TargetUnique$
// link "another target creature" must be answerable together. With one creature
// on the battlefield there is no legal choice, so the ability leaves the stack
// without asking; with a second, the ask names the Dinosaur.
func TestTriggerUniqueChainNeedsTwoCreatures(t *testing.T) {
	t.Parallel()
	script := "Name:Biter\nTypes:Creature Dinosaur\nPT:2/2\n" +
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Mark | TriggerDescription$ x\n" +
		"SVar:Mark:DB$ Pump | ValidTgts$ Dinosaur.YouCtrl | SubAbility$ Bite\n" +
		"SVar:Bite:DB$ DealDamage | ValidTgts$ Creature | TargetUnique$ True | NumDmg$ 1\nOracle:x\n"
	reg := testutil.CorpusRegistry(t)
	fire := func(t *testing.T, opponents []string) (*Engine, state.ObjID) {
		e, _ := combatTriggerBoard(t, reg, nil, []string{script}, nil, opponents)
		biter := findBattlefield(t, e, 0, "Biter", 0)
		e.pending = nil
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 3})
		e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
		e.drainTriggers()
		return e, biter
	}
	t.Run("alone", func(t *testing.T) {
		e, biter := fire(t, nil)
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			t.Fatalf("asked for a target with one creature on the battlefield: %+v", d.Options)
		}
		for _, id := range e.G.Stack {
			if o := e.G.Obj(id); o != nil && o.Source == biter {
				t.Fatalf("the ability stayed on the stack with no legal choice")
			}
		}
	})
	t.Run("two creatures", func(t *testing.T) {
		e, biter := fire(t, []string{"Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"})
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != biter {
			t.Fatalf("the root ask = %+v, want the Dinosaur alone", d)
		}
	})
}
