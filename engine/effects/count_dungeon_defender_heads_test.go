package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// countDungeonDefenderHeadsTest hosts the two SVar-body heads Ravenloft
// Adventurer's Attacks trigger reads:
//
//   - SVar:X is `DungeonsCompleted$Amount`, the gate's CheckSVar$ body — the
//     controller's completed-dungeon count, folded by events.Apply's
//     DungeonComplete.
//   - SVar:Y is `TriggeredDefendingPlayer$ValidExile
//     Card.YouOwn+counters_GE1_HIT`, the body's LifeAmount — the cards the
//     DEFENDING player owns in exile that carry a hit counter.
//
// Both used to fall through to a fail-closed zero, so the trigger never fired.

// TestDungeonsCompletedHead: the DungeonsCompleted$Amount body reads
// Player.CompletedDungeons — 0 before any dungeon was completed, one per
// DungeonComplete event, per resolving controller.
func TestDungeonsCompletedHead(t *testing.T) {
	h, c := fixtureHost(t)
	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || got != 0 {
		t.Fatalf("precondition: DungeonsCompleted$Amount = (%d, %v), want (0, true)", got, ok)
	}

	// Complete a dungeon for player 0 through the event fold (the
	// create/room/complete/remove sequence engine/rules' venture emits).
	d := mkCard(t, "Name:Lost Mine of Phandelver\nManaCost:no cost\nTypes:Dungeon\nK:Dungeon:Entrance,Temple\nOracle:dungeon\n")
	h.g.Tokens = map[string]*cards.Card{"lost_mine_of_phandelver": d}
	events.Apply(h.g, events.Event{Kind: events.DungeonCreate, Player: 0, Text: "lost_mine_of_phandelver"})
	id := h.g.Players[0].DungeonObj
	if id == 0 {
		t.Fatalf("precondition: DungeonCreate left no dungeon object")
	}
	h.Emit(events.Event{Kind: events.DungeonRoom, Player: 0, Obj: id, Text: "Entrance"})
	h.Emit(events.Event{Kind: events.DungeonRoom, Player: 0, Obj: id, Text: "Temple"})
	if h.g.Players[0].CompletedDungeons != 0 {
		t.Fatalf("precondition: two rooms deep, CompletedDungeons = %d, want 0", h.g.Players[0].CompletedDungeons)
	}
	h.Emit(events.Event{Kind: events.DungeonComplete, Player: 0, Obj: id})
	if h.g.Players[0].CompletedDungeons != 1 {
		t.Fatalf("precondition: after DungeonComplete, CompletedDungeons = %d, want 1", h.g.Players[0].CompletedDungeons)
	}

	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || got != 1 {
		t.Fatalf("after completion DungeonsCompleted$Amount = (%d, %v), want (1, true)", got, ok)
	}
	// The other seat has completed nothing.
	c.Controller = 1
	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || got != 0 {
		t.Fatalf("controller 1 DungeonsCompleted$Amount = (%d, %v), want (0, true)", got, ok)
	}
}

// TestDefendingPlayerValidExileHead: the TriggeredDefendingPlayer$ValidExile
// body counts the cards the FIRING trigger's defending player owns in exile
// that match the spec — one HIT-countered card owned by the defender reads 1,
// and the counter predicate is what the count filters on.
func TestDefendingPlayerValidExileHead(t *testing.T) {
	h, c := fixtureHost(t)
	card := mkCard(t, "Name:Bearer\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 1) // owned and controlled by the defender, seat 1
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZBattlefield, To: state.ZExile})
	if o.Zone != state.ZExile || o.Owner != 1 {
		t.Fatalf("precondition: exiled object zone=%s owner=%d, want Exile/1", o.Zone, o.Owner)
	}

	expr := "TriggeredDefendingPlayer$ValidExile Card.YouOwn+counters_GE1_HIT"
	c.TriggerContext.DefendingPlayer = state.Target{IsPlayer: true, Player: 1}
	if got, ok := EvalCountOK(h, c, expr); !ok || got != 0 {
		t.Fatalf("no counter yet: %q = (%d, %v), want (0, true)", expr, got, ok)
	}
	h.Emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "HIT", Amount: 1})
	if o.Counter("HIT") != 1 {
		t.Fatalf("precondition: hit counter = %d, want 1", o.Counter("HIT"))
	}

	// One HIT-countered exiled card owned by the defender reads 1.
	if got, ok := EvalCountOK(h, c, expr); !ok || got != 1 {
		t.Fatalf("with a hit counter %q = (%d, %v), want (1, true)", expr, got, ok)
	}

	// The count is scoped to the defending player: player 0 owns no such card.
	c.TriggerContext.DefendingPlayer = state.Target{IsPlayer: true, Player: 0}
	if got, ok := EvalCountOK(h, c, expr); !ok || got != 0 {
		t.Fatalf("defender 0 %q = (%d, %v), want (0, true)", expr, got, ok)
	}

	// The counter predicate binds: a hit-countered card owned by the OTHER
	// seat does not answer the defender's spec either.
	c.TriggerContext.DefendingPlayer = state.Target{IsPlayer: true, Player: 1}
	mine := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: mine.ID, From: state.ZBattlefield, To: state.ZExile})
	h.Emit(events.Event{Kind: events.CounterChange, Obj: mine.ID, Counter: "HIT", Amount: 1})
	if got, ok := EvalCountOK(h, c, expr); !ok || got != 1 {
		t.Fatalf("attacker-owned hit counter %q = (%d, %v), want (1, true)", expr, got, ok)
	}
}
