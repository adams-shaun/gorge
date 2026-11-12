package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// ConditionDefined$ TriggeredTargetLKICopy gates on the trigger's
// TriggerTarget role (the damaged recipient for DamageDone/DamageDoneOnce).
// It used to be an unmodelled group, so conditionMetCore failed it closed and
// The Incredible Hulk's enrage rider ("if he's attacking, untap him and there
// is an additional combat phase") never ran.

// hulkEnrageSrc is The Incredible Hulk's enrage face, written inline.
const hulkEnrageSrc = "Name:Enrage Hulk\nManaCost:2 R R G G\nTypes:Legendary Creature Gamma Berserker Hero\nPT:8/8\n" +
	"T:Mode$ DamageDoneOnce | Execute$ OnPutCounter | ValidTarget$ Card.Self | TriggerZones$ Battlefield\n" +
	"SVar:OnPutCounter:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1 | SubAbility$ Untap\n" +
	"SVar:Untap:DB$ Untap | Defined$ Self | ConditionDefined$ TriggeredTargetLKICopy | ConditionPresent$ Creature.attacking | SubAbility$ AddCombat\n" +
	"SVar:AddCombat:DB$ AddPhase | ExtraPhase$ Combat | AfterPhase$ EndCombat | ConditionDefined$ TriggeredTargetLKICopy | ConditionPresent$ Creature.attacking\n" +
	"Oracle:x\n"

func hulkEnrageBoard(t *testing.T, attacking bool) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg, nil, []string{hulkEnrageSrc}, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	hulk := findBattlefield(t, e, 0, "Enrage Hulk", 0)
	if attacking {
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{hulk}})
	}
	e.emit(events.Event{Kind: events.Tap, Obj: hulk})
	o := e.G.Obj(hulk)
	if o == nil || o.Zone != state.ZBattlefield || !o.Tapped || o.IsAttacking != attacking {
		t.Fatalf("precondition: hulk = %+v, want tapped battlefield creature with IsAttacking=%v", o, attacking)
	}
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("precondition: ExtraPhases = %v, want none", e.G.ExtraPhases)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: hulk, Amount: 2})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pending triggers after damage = %d, want 1", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	// The head of the chain ran either way: the gate is on the subs only.
	if got := e.G.Obj(hulk).Counter("P1P1"); got != 1 {
		t.Fatalf("P1P1 counters = %d, want 1 (the enrage trigger did not resolve)", got)
	}
	return e, cfg, hulk
}

func TestHulkEnrageAttackingUntapsAndAddsCombat(t *testing.T) {
	t.Parallel()
	e, cfg, hulk := hulkEnrageBoard(t, true)
	if e.G.Obj(hulk).Tapped {
		t.Fatalf("attacking hulk still tapped: the TriggeredTargetLKICopy gate denied the untap")
	}
	if len(e.G.ExtraPhases) != 1 {
		t.Fatalf("ExtraPhases = %v, want one extra combat", e.G.ExtraPhases)
	}
	replayCheck(t, e, cfg)
}

func TestHulkEnrageNotAttackingDoesNothingMore(t *testing.T) {
	t.Parallel()
	e, cfg, hulk := hulkEnrageBoard(t, false)
	if !e.G.Obj(hulk).Tapped {
		t.Fatalf("non-attacking hulk untapped: the gate must deny when he is not attacking")
	}
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("ExtraPhases = %v, want none for a non-attacking hulk", e.G.ExtraPhases)
	}
	replayCheck(t, e, cfg)
}
