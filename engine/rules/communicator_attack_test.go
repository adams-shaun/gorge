package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// combadgeAttack equips a Combadge to each of seat 0's first `equipped`
// Grizzly Bears, declares the first bear as the only attacker, resolves its
// triggers, and reports whether seat 1's Hill Giant can still block it.
func combadgeAttack(t *testing.T, equipped int) bool {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	bears, badge := lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Combadge")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{bears, bears, badge, badge}, []*cards.Card{lookup(t, reg, "Hill Giant")})
	var ids []state.ObjID
	for i := 0; i < 2; i++ {
		ids = append(ids, moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield))
	}
	for i := 0; i < equipped; i++ {
		b := moveByName(t, e, 0, "Combadge", state.ZBattlefield)
		e.emit(events.Event{Kind: events.Attach, Obj: b, IDs: []state.ObjID{ids[i]}})
		if e.G.Obj(b).AttachedTo != ids[i] {
			t.Fatalf("precondition: Combadge %d is not attached to bear %d", b, ids[i])
		}
	}
	giant := moveByName(t, e, 1, "Hill Giant", state.ZBattlefield)
	e.G.Obj(ids[0]).SummonSick = false
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{ids[0]}})
	drainEtb(t, e)
	return e.canBlock(giant, ids[0])
}

// TestCombadgeAttackerUnblockableWithAnotherCommunicator: Combadge's granted
// "whenever this creature attacks while you control another creature with a
// Communicator attached to it, this creature can't be blocked this turn"
// (IsPresent$ Card.Communicator+AttachedTo Creature.Other+YouCtrl, the
// Other read against the attacking creature that has the trigger) makes the
// attacker unblockable only while a second equipped creature is there: its
// own Combadge is attached to it, not to another creature (CR 603.4).
func TestCombadgeAttackerUnblockableWithAnotherCommunicator(t *testing.T) {
	if !combadgeAttack(t, 1) {
		t.Fatal("a lone equipped attacker became unblockable: its own Combadge counted as another creature's")
	}
	if combadgeAttack(t, 2) {
		t.Fatal("an attacker with another equipped creature beside it stayed blockable")
	}
}
