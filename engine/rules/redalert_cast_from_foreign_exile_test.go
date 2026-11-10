package rules

// redalert_cast_from_foreign_exile_test.go pins whose attackers Red Alert!'s
// own cost static counts when the spell is cast from another player's exile
// (CR 601.2a, 601.2f, 109.5). "This spell costs {2} less to cast if a
// creature is attacking you": the spell is on the stack under the caster's
// control when its cost is worked out, so "you" is the caster, never the
// card's owner. Ian Malcolm, Chaotician's real trigger puts seat 1's Red
// Alert! into exile with Ian, and seat 0 (the active player) may cast it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestRedAlertFromForeignExileCountsAttackersAtTheCaster(t *testing.T) {
	e, _, ids := mpRiderGame(t, nil, []string{"Red Alert!"},
		[]string{"Ian Malcolm, Chaotician", "Savannah Lions"}, nil)
	driveToTurn3Main(t, e)
	ian := ids[0]["Ian Malcolm, Chaotician"][0]
	lions := ids[0]["Savannah Lions"][0]
	alert := ids[1]["Red Alert!"][0]

	// Seat 1's own second draw exiles Red Alert! with Ian; on seat 0's next
	// turn seat 0 may cast it with one {R} in its pool: {2}{R} unreduced is
	// out of reach, {R} reduced is not.
	ianAt(t, e, 4, 1, "R")
	ianExileSecondDraw(t, e, 1, ian, alert)
	ianAt(t, e, 5, 0, "R")
	if o := e.G.Obj(alert); o.Zone != state.ZExile || o.Owner != 1 || o.ExiledWith != ian {
		t.Fatalf("precondition: Red Alert! zone %s owner %d exiledWith %d, want seat 1's card exiled with Ian", o.Zone, o.Owner, o.ExiledWith)
	}
	if e.G.Players[0].Pool.Total() != 1 {
		t.Fatalf("precondition: seat 0 pool %d, want exactly one mana", e.G.Players[0].Pool.Total())
	}
	if ianOffers(e, alert) {
		t.Fatalf("precondition: Red Alert! offered with no attacker and one mana: %v", mpRiderLabels(e))
	}

	// Seat 0 attacks seat 1: a creature attacks the card's owner, not the
	// caster.
	l := e.G.Obj(lions)
	if l.Zone != state.ZBattlefield || l.Controller != 0 {
		t.Fatalf("precondition: attacker %+v not on seat 0's battlefield", l)
	}
	l.SummonSick = false
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{lions}})
	e.G.Step = state.StepDeclareAttackers
	e.priorityRound()
	if l = e.G.Obj(lions); !l.IsAttacking || l.Attacking != 1 {
		t.Fatalf("precondition: Lions attacking %v at seat %d, want attacking seat 1", l.IsAttacking, l.Attacking)
	}
	if d := e.Pending(); d == nil || d.Player != 0 || e.G.Step != state.StepDeclareAttackers {
		t.Fatalf("precondition: want seat 0's priority in declare attackers, got %+v at %s", d, e.G.Step)
	}
	if e.G.Players[0].Pool.Total() != 1 {
		t.Fatalf("precondition: seat 0 pool %d after the declaration, want one mana", e.G.Players[0].Pool.Total())
	}

	if ianOffers(e, alert) {
		t.Fatalf("Red Alert! offered to seat 0 for one mana though nothing attacks seat 0: %v", mpRiderLabels(e))
	}
}
