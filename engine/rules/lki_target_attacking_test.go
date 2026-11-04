package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A DamageDone trigger's ConditionDefined$ TriggeredTargetLKICopy gate must
// read the damaged object's attacking status from the last battlefield
// instant, not from the live object after a lethal-damage Move (CR 608.2h).
// The Incredible Hulk's enrage rider ("if he's attacking, untap him and there
// is an additional combat phase after this phase") is the corpus shape: the
// Move that kills him clears IsAttacking before the trigger resolves, so a
// live read denies the rider and the extra combat phase never happens.
func TestHulkEnrageStillAttackingAfterLethalDamage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg, nil, []string{hulkEnrageSrc}, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	hulk := findBattlefield(t, e, 0, "Enrage Hulk", 0)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{hulk}})
	if o := e.G.Obj(hulk); o == nil || o.Zone != state.ZBattlefield || !o.IsAttacking {
		t.Fatalf("precondition: hulk = %+v, want an attacking battlefield creature", o)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: hulk, Amount: 8})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending triggers after lethal damage = %d, want 1", len(e.pendingTriggers))
	}
	e.checkStateBased()
	o := e.G.Obj(hulk)
	if o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: hulk = %+v, want it moved to the graveyard by lethal damage", o)
	}
	if o.IsAttacking {
		t.Fatal("precondition: the live graveyard object still says attacking, so a live read could pass")
	}
	e.putTriggersOnStack()
	e.resolveTop()
	if len(e.G.ExtraPhases) != 1 {
		t.Fatalf("ExtraPhases = %v, want the one extra combat the dead attacker's enrage rider grants", e.G.ExtraPhases)
	}
	replayCheck(t, e, cfg)
}
