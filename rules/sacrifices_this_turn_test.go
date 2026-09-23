package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestSacrificedThisTurnRemainsUnresolvedWithoutActorProvenance(t *testing.T) {
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	creature := card(t, "Name:Creature\nTypes:Creature\nPT:2/2\nOracle:x\n")
	permanent := e.G.AddObject(creature, 1)
	permanent.Controller = 0
	permanent.Zone = state.ZBattlefield
	if permanent.Owner != 1 || permanent.Controller != 0 || permanent.Zone != state.ZBattlefield {
		t.Fatalf("precondition: need a battlefield permanent owned by player 1 and controlled by player 0: %+v", permanent)
	}

	// Model player 0 sacrificing a permanent owned by player 1. The event
	// contains no actor, so neither owner nor controller is a sound count.
	e.emit(events.Sacrifice(permanent.ID))
	if !events.IsSacrifice(e.L.Events[len(e.L.Events)-1]) {
		t.Fatal("precondition: expected a canonical sacrifice event")
	}
	if got, ok := effects.EvalCountOK(e, &effects.Ctx{Controller: 0}, "PlayerCountPropertyYou$SacrificedThisTurn"); ok || got != 0 {
		t.Fatalf("player 0 SacrificedThisTurn = (%d, %v), want unresolved (0, false) without event actor provenance", got, ok)
	}
}
