package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestMultiWordSubtypePredicate(t *testing.T) {
	makeObject := func(types string) *state.Object {
		c, diags := cards.ParseBytes("inline.txt", []byte("Name:Test\nTypes:Creature "+types+"\nOracle:x\n"))
		if len(diags) != 0 {
			t.Fatalf("parse inline card: %v", diags)
		}
		return &state.Object{ID: 1, Card: c, Controller: 0, Owner: 0, Zone: state.ZBattlefield}
	}
	both := makeObject("Time Lord")
	withoutTime := makeObject("Lord")
	withoutLord := makeObject("Time")
	for _, tc := range []struct {
		label string
		obj   *state.Object
	}{{"both", both}, {"without Time", withoutTime}, {"without Lord", withoutLord}} {
		if tc.obj.Zone != state.ZBattlefield {
			t.Fatalf("%s precondition: zone = %v, want battlefield", tc.label, tc.obj.Zone)
		}
	}
	if !MatchesObjectCtx(nil, "Card.Time Lord+YouCtrl", both, SpecContext{You: 0}) {
		t.Fatal("Card.Time Lord+YouCtrl must match an object carrying both constituent type words")
	}
	if MatchesObjectCtx(nil, "Card.Time Lord+YouCtrl", withoutTime, SpecContext{You: 0}) {
		t.Fatal("predicate matched without Time")
	}
	if MatchesObjectCtx(nil, "Card.Time Lord+YouCtrl", withoutLord, SpecContext{You: 0}) {
		t.Fatal("predicate matched without Lord")
	}
	if un := UnknownPredicates("Card.Time Lord+YouCtrl"); len(un) != 0 {
		t.Fatalf("supported token reported unknown: %v", un)
	}
	unknown := "Card.Time Mystery+YouCtrl"
	if MatchesObjectCtx(nil, unknown, both, SpecContext{You: 0}) {
		t.Fatal("unsupported multiword predicate matched")
	}
	if un := UnknownPredicates(unknown); len(un) != 1 || un[0] != "Time Mystery" {
		t.Fatalf("UnknownPredicates(%q) = %v, want [Time Mystery]", unknown, un)
	}
	if MatchesObjectCtx(nil, "Card.nonTime Mystery+YouCtrl", both, SpecContext{You: 0}) {
		t.Fatal("unknown multiword predicate under negation must fail closed")
	}
}
