package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmPortRazer: after Port Razer has attacked p1 this turn, its own
// CantAttack static must refuse every later combat's p1 pair; the
// controller's other creature must still be offered (the history is
// per-source). Fails on the pin because Player.attackedBySourceThisTurn is
// an unknown qualifier.
func TestFarmPortRazer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Port Razer")
	if !ok {
		t.Fatal("corpus missing Port Razer")
	}
	var target string
	for _, s := range c.Faces[0].Statics {
		if s.Mode == "CantAttack" {
			target = s.Params["Target"]
		}
	}
	if target != "Player.attackedBySourceThisTurn" {
		t.Fatalf("precondition: Port Razer CantAttack Target$ = %q", target)
	}

	other := card(t, staticBearFixture)
	e, _ := restrictionGame(t, 6201,
		[][]*cards.Card{nil, nil},
		[][]*cards.Card{{c, other}, {}})
	razer := bearOnBoard(t, e, 0, c)
	bear := bearOnBoard(t, e, 0, other)

	if e.attackBlocked(razer, 1, 0) {
		t.Fatal("Port Razer was blocked before it attacked p1")
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{razer}})
	e.emit(events.Event{Kind: events.EndCombatReset}) // the first combat ends; the extra combat begins
	if !e.attackBlocked(razer, 1, 0) {
		t.Fatal("Port Razer can attack p1 again in an additional combat: the qualifier did not resolve")
	}
	if e.attackBlocked(razer, 0, 0) {
		t.Fatal("Port Razer has not attacked p0")
	}
	if e.attackBlocked(bear, 1, 0) {
		t.Fatal("the controller's other creature was blocked: the history must be per-source, not per-controller")
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	if e.attackBlocked(razer, 1, 0) {
		t.Fatal("the per-turn history survived TurnChange")
	}
}
