package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGraveyardSelfReturnFindsOnlyTheSameObject pins CR 400.7: Reassembling
// Skeleton's "{1}{B}: Return this card from your graveyard to the
// battlefield tapped" names the card that was in the graveyard when the
// ability was activated. If that card goes to the battlefield and back to
// the graveyard before the ability resolves (an earlier activation returned
// it, then it was sacrificed), the card in the graveyard is a new object and
// the ability does nothing. The stack object's push stamp now records the
// source's Incarnation whatever its zone (events.stampPushedAbilitySources),
// and effects.sourceReturnedSincePush skips the move. dirac's
// item (pair 88 and ~70 of the 10k slice). The control arm resolves with no
// round trip and must return the Skeleton.
func TestGraveyardSelfReturnFindsOnlyTheSameObject(t *testing.T) {
	t.Parallel()
	for _, roundTrip := range []bool{false, true} {
		skel := corpusAlternativeCard(t, "Reassembling Skeleton")
		e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{skel})
		var id state.ObjID
		for i := range e.G.Objs {
			if e.G.Objs[i].Card == skel {
				id = e.G.Objs[i].ID
			}
		}
		if id == 0 {
			t.Fatal("setup: no Skeleton")
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
		toMain1(t, e)
		addMana(t, e, 0, "BB")
		submitChoices(t, e, abilityOption(t, e, id, 0).Index)
		if len(e.G.Stack) != 1 {
			t.Fatalf("precondition: one ability on the stack, got %d", len(e.G.Stack))
		}
		if roundTrip {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZBattlefield})
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
		}
		passUntilStackEmpty(t, e, 20)
		want := state.ZBattlefield
		if roundTrip {
			want = state.ZGraveyard
		}
		if z := e.G.Obj(id).Zone; z != want {
			t.Fatalf("roundTrip=%v: Skeleton zone %v, want %v", roundTrip, z, want)
		}
		replayCheck(t, e, cfg)
	}
}
