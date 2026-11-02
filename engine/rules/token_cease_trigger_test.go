package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestATokenCeasingDoesNotLeaveAZone pins CR 111.7-8: a token in the graveyard
// ceases to exist, which is no zone change, so Chalk Outline's "creature cards
// leave your graveyard" stays silent and the count of cards that left doesn't
// see it. A creature card leaving still fires the trigger and is counted.
func TestATokenCeasingDoesNotLeaveAZone(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	eng, cfg := miscHandsEngine(t, reg, []string{"Grizzly Bears"}, nil, []string{"Chalk Outline"}, nil)
	eng.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	bf := eng.G.Zone(state.ZBattlefield, 0)
	tok := bf[len(bf)-1]
	eng.emit(events.Event{Kind: events.Damage, Obj: tok, Amount: 1})
	eng.checkStateBased()
	if got := eng.G.Obj(tok).Zone; got != state.ZCeased {
		t.Fatalf("token zone = %s, want ceased", got)
	}
	eng.putTriggersOnStack()
	if len(eng.G.Stack) != 0 {
		t.Fatalf("a token ceasing put %v on the stack", eng.G.Stack)
	}

	card := miscHandObj(t, eng, 0, "Grizzly Bears")
	eng.emit(events.Event{Kind: events.MoveZone, Obj: card, From: state.ZHand, To: state.ZGraveyard})
	eng.emit(events.Event{Kind: events.MoveZone, Obj: card, From: state.ZGraveyard, To: state.ZExile})
	eng.putTriggersOnStack()
	if len(eng.G.Stack) != 1 {
		t.Fatalf("a creature card leaving the graveyard: stack %v, want the trigger", eng.G.Stack)
	}
	if n := evalHead(t, eng, 0, 0, "Count$LeftGraveyardThisTurn Card.YouOwn"); n != 1 {
		t.Fatalf("LeftGraveyardThisTurn = %d, want 1: the creature card, not the token", n)
	}
	replayCheck(t, eng, cfg)
}
