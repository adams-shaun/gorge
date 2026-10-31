package botpolicy

import (
	"fmt"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A permanent that makes mana may be needed for the rest of the payment, so the bot
// taps the ones that don't first and tops up in offer order only when it must.
func TestBotTapsHelpersThatMakeNoManaFirst(t *testing.T) {
	var b Board
	b.Cards.Set(1, Card{Produces: cards.ManaProduction{Colour: [6]int32{5: 1}}})
	b.Cards.Set(2, Card{})
	b.Cards.Set(3, Card{Produces: cards.ManaProduction{Any: true}})
	b.Cards.Set(4, Card{})
	opts := func(kind string) []decision.Option {
		var out []decision.Option
		for i, id := range []state.ObjID{1, 2, 3, 4} {
			out = append(out, decision.Option{Index: i, Kind: kind, Obj: id, Group: fmt.Sprintf("payment:%d", id)})
		}
		return out
	}
	for _, tc := range []struct {
		kind string
		min  int
		want []int
	}{
		{"waterbend_generic", 2, []int{1, 3}},
		{"waterbend_generic", 3, []int{1, 3, 0}},
		{"tapcost", 1, []int{1}},
		{"convoke_generic", 0, []int{1}},
	} {
		d := &decision.Decision{Seq: 1, Kind: decision.KChoose, Min: tc.min, Max: 4, Options: opts(tc.kind)}
		in := Decide(b, d, rng(1))
		if !slices.Equal(in.Choices, tc.want) {
			t.Errorf("%s min %d: choices %v, want %v", tc.kind, tc.min, in.Choices, tc.want)
		}
		if err := d.Validate(in); err != nil {
			t.Errorf("%s min %d: answer rejected: %v", tc.kind, tc.min, err)
		}
	}
}
