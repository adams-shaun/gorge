package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLoyaltyActivationUsesCopyFaceAtPush proves the CR 606.3 scan classifies
// an AbilityPush by the copy basis active WHEN it was pushed, not the copy
// basis the permanent has at query time. The second copy basis puts its
// loyalty ability at no index, so a current-face lookup loses the activation.
func TestLoyaltyActivationUsesCopyFaceAtPush(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	loyaltySrc := &cards.Card{Faces: []*cards.Face{{
		Name: "loyalty source", Types: []string{"Creature"},
		Abilities: []*cards.SA{{Kind: "AB", API: "Draw", Params: map[string]string{"Planeswalker": "True"}}},
	}}}
	otherSrc := &cards.Card{Faces: []*cards.Face{{
		Name: "other source", Types: []string{"Creature"},
		Abilities: []*cards.SA{{Kind: "AB", API: "Draw"}},
	}}}
	e, _, walker := walkerBoard(t, reg, "Jace, the Mind Sculptor", loyaltySrc, otherSrc)
	// The walker's printed face has NO loyalty ability at index 0 (the push's
	// Amount), so only the copy basis in force at the push can count it: a
	// fold that reads the printed face finds a plain Draw there.
	e.G.Obj(walker).Card = &cards.Card{Faces: []*cards.Face{{
		Name: "plain walker", Types: []string{"Planeswalker"}, Loyalty: "3",
		Abilities: []*cards.SA{{Kind: "AB", API: "Draw", Params: map[string]string{}}},
	}}}
	e.G.Obj(walker).SyncFace()
	find := func(c *cards.Card) state.ObjID {
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Owner == 0 && o.Card == c && o.Zone == state.ZBattlefield {
				return o.ID
			}
		}
		t.Fatalf("no %q on the battlefield", c.Faces[0].Name)
		return 0
	}
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: walker, IDs: []state.ObjID{find(loyaltySrc)}})
	recordLoyaltyPush(e, walker, 0)
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: walker, IDs: []state.ObjID{find(otherSrc)}})
	if f := e.G.Obj(walker).Card.Faces[0]; e.isLoyaltyAbility(f.Abilities[0]) {
		t.Fatal("precondition: the printed face's ability 0 must not be a loyalty ability")
	}
	if got := e.loyaltyActivationsThisTurn(walker); got != 1 {
		t.Fatalf("loyalty activations after a copy-again = %d, want 1 (the push was on the first copy basis)", got)
	}
}
