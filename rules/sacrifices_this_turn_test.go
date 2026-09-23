package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestSacrificesThisTurnCountsOwnedPermanentsAndResets(t *testing.T) {
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	creature := card(t, "Name:Creature\nTypes:Creature\nPT:2/2\nOracle:x\n")
	owned := e.G.AddObject(creature, 0)
	owned.Zone = state.ZBattlefield
	controlledByOther := e.G.AddObject(creature, 1)
	controlledByOther.Zone = state.ZBattlefield
	if owned.Zone != state.ZBattlefield || controlledByOther.Zone != state.ZBattlefield {
		t.Fatal("precondition: sacrifice candidates must be on the battlefield")
	}
	e.emit(events.Sacrifice(owned.ID))
	e.emit(events.Sacrifice(controlledByOther.ID))
	if got := e.SacrificesThisTurn(0); got != 1 {
		t.Fatalf("owner 0 sacrifices = %d, want 1", got)
	}
	if got := e.SacrificesThisTurn(1); got != 1 {
		t.Fatalf("owner 1 sacrifices = %d, want 1", got)
	}
	e.emit(events.Event{Kind: events.TurnChange})
	if got := e.SacrificesThisTurn(0); got != 0 {
		t.Fatalf("after TurnChange, owner 0 sacrifices = %d, want 0", got)
	}
}
