package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPlayerSpecAttackedYouTheirLastTurn drives the REAL corpus card
// O-Kagachi, Vengeful Kami's "deals combat damage to a player, if that player
// attacked you during their last turn" trigger (ValidTarget$
// Player.attackedYouTheirLastTurn) through trigmatch.DamageMatches. The
// qualifier failed closed before, so the trigger never fired. It reads the
// damaged seat's most recent COMPLETED turn: an attack in the turn in
// progress does not count yet, and a later turn of that seat without an
// attack clears it. At every step it agrees with Engine.AttackedDuringLastTurn
// (Avenge's count), the event-log reading of the same property.
func TestPlayerSpecAttackedYouTheirLastTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := dsBoardWith(t, reg, mustCorpusCard(t, reg, "Grizzly Bears"), "O-Kagachi, Vengeful Kami")
	oka := ids["O-Kagachi, Vengeful Kami"]
	bear := fx20BattlefieldByName(t, e, 1, "Grizzly Bears")
	if e.G.Obj(oka).Zone != state.ZBattlefield || e.controllerOf(oka) != 0 || e.controllerOf(bear) != 1 {
		t.Fatal("precondition: O-Kagachi on seat 0's battlefield, the bear on seat 1's")
	}
	trig := fx20TriggerWithParam(t, e, oka, "Player.attackedYouTheirLastTurn")
	e.combatDamaging = true
	e.dmgSrcOverride = oka
	turn := e.G.Turn
	step := func(what string, ev events.Event, want bool) {
		t.Helper()
		if ev.Kind == events.TurnChange {
			turn++
			ev.Amount = turn
		}
		e.emit(ev)
		e.pending = nil
		got := trigmatch.DamageMatches(boardOf(e), trig, oka, events.Event{Kind: events.Damage, Player: 1, Amount: 6})
		if got != want {
			t.Fatalf("%s: the trigger matched = %v, want %v", what, got, want)
		}
		if log := e.AttackedDuringLastTurn(1, 0); log != want {
			t.Fatalf("%s: AttackedDuringLastTurn(1, 0) = %v, the state read %v", what, log, got)
		}
	}
	step("seat 0's turn, before seat 1 has attacked", events.Event{Kind: events.TurnChange, Player: 0}, false)
	step("seat 1's turn begins", events.Event{Kind: events.TurnChange, Player: 1}, false)
	step("seat 1 attacks seat 0 in its turn in progress", events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{bear}}, false)
	step("seat 0's next turn", events.Event{Kind: events.TurnChange, Player: 0}, true)
	step("seat 1's next turn, in progress", events.Event{Kind: events.TurnChange, Player: 1}, true)
	step("seat 0's turn after seat 1's peaceful turn", events.Event{Kind: events.TurnChange, Player: 0}, false)
}
