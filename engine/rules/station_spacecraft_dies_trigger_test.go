package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 603.10a, 704.3: a stationed Spacecraft (a creature through its STATION 4+
// static) that dies in the same state-based check that eliminates its
// controller is looked back at as the creature it was, so "whenever a creature
// dies" triggers. The engine is built with the cards in the decks, as a real
// game is, so the pool gates are the game's own.
func TestStationedSpacecraftDiesTriggersCreatureDies(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	blade := corpusAlternativeCard(t, "Blade of the Bloodchief")
	ship := corpusAlternativeCard(t, "U.S.S. Prometheus, Prototype")
	e := corpusEngine(t, reg, []*cards.Card{blade}, []*cards.Card{ship})
	if !e.layer4InPool {
		t.Fatal("precondition: the Spacecraft's STATION static must raise layer4InPool from the decks")
	}
	putOnBattlefield(t, e, 0, blade)
	id := putOnBattlefield(t, e, 1, ship)
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 8})
	if !e.IsCreature(id) {
		t.Fatal("precondition: the stationed Spacecraft must be a creature")
	}
	e.G.Obj(id).Damage = 6
	e.G.Players[1].Life = 0
	before := len(e.pendingTriggers)
	e.checkStateBased()
	if !e.G.Players[1].Lost {
		t.Fatal("precondition: the controller must have lost in the same check")
	}
	if z := e.G.Obj(id).Zone; z == state.ZBattlefield {
		t.Fatal("precondition: the Spacecraft must have died")
	}
	if got := len(e.pendingTriggers) - before; got != 1 {
		t.Errorf("%d dies triggers queued for the stationed Spacecraft, want 1", got)
	}
}
