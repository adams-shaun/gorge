package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestMultiWordSubtypePredicate(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	makeObject := func(types ...string) *state.Object {
		return &state.Object{Card: &cards.Card{Faces: []*cards.Face{{Types: types}}}, Controller: 0, Zone: state.ZBattlefield}
	}
	both := makeObject("Creature", "Time", "Lord")
	withoutLord := makeObject("Creature", "Time")
	withoutTime := makeObject("Creature", "Lord")
	if both.Zone != state.ZBattlefield || withoutLord.Zone != state.ZBattlefield || withoutTime.Zone != state.ZBattlefield {
		t.Fatal("test objects must be on the battlefield")
	}
	ctx := SpecContext{You: 0}
	for _, tc := range []struct {
		name string
		o    *state.Object
		want bool
	}{{"both words", both, true}, {"missing Lord", withoutLord, false}, {"missing Time", withoutTime, false}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchesObjectCtx(g, "Card.Time Lord", tc.o, ctx); got != tc.want {
				t.Fatalf("Card.Time Lord = %v, want %v", got, tc.want)
			}
		})
	}
	if unknown := UnknownPredicates("Card.Time Lord"); len(unknown) != 0 {
		t.Fatalf("supported token reported unknown: %v", unknown)
	}
	unknownSpec := "Card.Time Lordish"
	if got := MatchesObjectCtx(g, unknownSpec, both, ctx); got {
		t.Fatal("unsupported multi-word predicate must fail closed")
	}
	if unknown := UnknownPredicates(unknownSpec); len(unknown) != 1 || unknown[0] != "Time Lordish" {
		t.Fatalf("UnknownPredicates(%q) = %v", unknownSpec, unknown)
	}
	// TARDIS's intervening-if uses this exact IsPresent predicate shape.
	if got := MatchesObjectCtx(g, "Creature.YouCtrl+Time Lord", both, ctx); !got {
		t.Fatal("TARDIS-shaped IsPresent$ filter must find a controlled Time Lord")
	}
}
