package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGraveyardActivationCopyFindsOnlyTheSameObject pins CR 707.10 + 400.7:
// Project Deathlok Soldier's "{2}{B}: Return this card from your graveyard
// to your hand" names the card that was in the graveyard when the ability was
// activated. A card that went graveyard -> hand -> graveyard before the
// ability resolves is a new object (it never crossed the battlefield), so the
// ability does nothing -- and a copy of the ability (Echo, Perceptive
// Prodigy) has the same source and finds nothing either. The control arms
// resolve with no round trip and must return the card.
func TestGraveyardActivationCopyFindsOnlyTheSameObject(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		roundTrip, copy bool
	}{
		{"original-control", false, false},
		{"copy-control", false, true},
		{"original-round-trip", true, false},
		{"copy-round-trip", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			soldier := corpusAlternativeCard(t, "Project Deathlok Soldier")
			e, cfg, _ := corpusDeckEngine(t, nil, []*cards.Card{soldier})
			var id state.ObjID
			for i := range e.G.Objs {
				if e.G.Objs[i].Card == soldier {
					id = e.G.Objs[i].ID
				}
			}
			if id == 0 {
				t.Fatal("setup: no Soldier")
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
			toMain1(t, e)
			addMana(t, e, 0, "BBB")
			submitChoices(t, e, abilityOption(t, e, id, 0).Index)
			if len(e.G.Stack) != 1 {
				t.Fatalf("precondition: one ability on the stack, got %d", len(e.G.Stack))
			}
			if tc.roundTrip {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZHand})
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
			}
			// The copy is minted after the round trip, as Echo's copy was: it
			// must carry the original's push stamp, not take its own now.
			if tc.copy {
				e.emit(events.Event{Kind: events.StackCopy, Obj: e.G.Stack[0], Player: 0})
				if len(e.G.Stack) != 2 {
					t.Fatalf("precondition: ability and its copy on the stack, got %d", len(e.G.Stack))
				}
			}
			passUntilStackEmpty(t, e, 20)
			want := state.ZHand
			if tc.roundTrip {
				want = state.ZGraveyard
			}
			if z := e.G.Obj(id).Zone; z != want {
				t.Fatalf("Soldier zone %v, want %v", z, want)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestSourceDiscardedByItsOwnCostIsStillFound pins the other half of the
// return stamp: Kogla and Yidaro's ability is activated from hand and discards
// the card as its cost, so the source is in the graveyard -- a zone it was not
// in at the push -- when "shuffle CARDNAME into your library" resolves. Only a
// return to the push zone is a new object (CR 400.7); a cost that moved the
// source elsewhere is not, and the ability still finds the card.
func TestSourceDiscardedByItsOwnCostIsStillFound(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	kogla := mustCorpusCard(t, reg, "Kogla and Yidaro")
	e := handEngine(t, kogla)
	id := e.G.Zone(state.ZHand, 0)[0]
	if e.G.Obj(id).Zone != state.ZHand {
		t.Fatalf("precondition: the source is in %v, want the hand at the push", e.G.Obj(id).Zone)
	}
	addMana(t, e, 0, "CCRG")
	var opt decision.Option
	found := false
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id && strings.Contains(o.Cost, "Discard<1/CARDNAME>") {
			opt, found = o, true
		}
	}
	if !found {
		t.Fatal("precondition: the discard activation is not offered")
	}
	e.beginActivation(0, opt)
	drainStackPassing(t, e, 40)
	switch z := e.G.Obj(id).Zone; z {
	case state.ZGraveyard, state.ZCeased:
		t.Fatalf("Kogla and Yidaro in %v: the cost's zone change was read as a return", z)
	}
}
