package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAutonSoldierCopyMyriadFires is CR 707.9b and 702.116a on the copy
// exception "except ... it has myriad": Auton Soldier entered as a copy of
// Isamaru attacks, and the copy's own Myriad trigger (folded into its
// copiable values) goes on the stack, sourced from the copy, and resolves in
// a two-player game asking only for priority.
func TestAutonSoldierCopyMyriadFires(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1422, []string{"Auton Soldier", "Isamaru, Hound of Konda"}, nil, nil)
	dog := findCardObj(t, e, 0, "Isamaru, Hound of Konda", state.ZBattlefield)
	as := findCardObj(t, e, 0, "Auton Soldier", state.ZHand)
	addMana(t, e, 0, "CCCCUU")
	castCopyCard(t, e, as, dog, 0)
	o := e.G.Obj(as)
	// Precondition: the copy is on the battlefield as Isamaru and has myriad;
	// a card that entered as itself has no Myriad trigger to fire.
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Isamaru, Hound of Konda" || !e.HasKeyword(as, "Myriad") {
		t.Fatalf("Auton Soldier did not enter as an Isamaru copy with myriad: %+v", o)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: stack depth %d before the attack, want 0", len(e.G.Stack))
	}

	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{as}})
	e.priorityRound()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack depth %d after the attack, want exactly the copy's Myriad trigger", len(e.G.Stack))
	}
	top := e.G.Obj(e.G.Stack[0])
	if top == nil || top.Ability == nil || top.Ability.API != "Myriad" || top.Source != as {
		t.Fatalf("stack object = %+v, want a Myriad ability sourced from the Auton Soldier copy", top)
	}
	passUntilStackEmpty(t, e, 40)
	replayCheck(t, e, cfg)
}
