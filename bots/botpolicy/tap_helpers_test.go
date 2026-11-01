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

// A Convoke announcement the seat's mana can't finish taps what the cast needs,
// and a plain v2 seat's rebuilt ask, whose offers name only the object, answers
// with the same objects.
func TestCastHelpersPriceTheSpellOnTheStack(t *testing.T) {
	const (
		bears, piker, lions, awakener, island state.ObjID = 1, 2, 3, 4, 5
		spell                                 state.ObjID = 9
	)
	green, red, white := cards.ColourMaskGreen, cards.ColourMaskRed, cards.ColourMaskWhite
	colours := map[state.ObjID]cards.ColourMask{bears: green, piker: red, lions: white, awakener: red | white}
	native := func(ids []state.ObjID) []decision.Option {
		var out []decision.Option
		add := func(id state.ObjID, kind string) {
			out = append(out, decision.Option{Index: len(out), Kind: kind, Obj: id, Group: fmt.Sprintf("payment:%d", id)})
		}
		for _, id := range ids {
			if colours[id]&white != 0 {
				add(id, "convoke_W")
			}
			if colours[id]&red != 0 {
				add(id, "convoke_R")
			}
			add(id, "convoke_generic")
		}
		return out
	}
	rebuilt := func(ids []state.ObjID) []decision.Option {
		var out []decision.Option
		for i, id := range ids {
			out = append(out, decision.Option{Index: i, Kind: "convoke", Obj: id})
		}
		return out
	}
	for _, tc := range []struct {
		name    string
		pool    state.Mana
		sources []state.ObjID // untapped lands making blue
		offer   []state.ObjID
		want    []state.ObjID
	}{
		{"the pool pays it, so the first helper as ever", state.Mana{1, 0, 0, 1, 1, 0}, nil, []state.ObjID{bears}, []state.ObjID{bears}},
		{"one short of generic", state.Mana{1, 0, 0, 1, 0, 0}, nil, []state.ObjID{bears}, []state.ObjID{bears}},
		{"a pip the pool lacks, not the creature that pays generic", state.Mana{1, 0, 0, 0, 1, 0}, nil, []state.ObjID{bears, piker}, []state.ObjID{piker}},
		{"no mana at all", state.Mana{}, nil, []state.ObjID{bears, piker, lions}, []state.ObjID{bears, piker, lions}},
		{"a two-colour creature pays one pip, its twin the other", state.Mana{0, 0, 0, 0, 1, 0}, nil, []state.ObjID{awakener, awakener + 10}, []state.ObjID{awakener, awakener + 10}},
		{"an untapped land still pays generic", state.Mana{1, 0, 0, 0, 0, 0}, []state.ObjID{island}, []state.ObjID{bears, piker}, []state.ObjID{piker}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b Board
			colours[awakener+10] = colours[awakener]
			b.Pool = tc.pool
			b.Stack = []StackEntry{{ID: spell, IsSpell: true, ManaCost: "X 1 R W"}}
			for _, id := range tc.offer {
				b.PublicCards.Set(id, PublicCard{Colour: colours[id]})
				b.Cards.Set(id, Card{OnBattlefield: true, Creature: true})
			}
			for _, id := range tc.sources {
				b.Cards.Set(id, Card{OnBattlefield: true, Produces: cards.ManaProduction{Colour: [6]int32{1: 1}}})
			}
			for shape, opts := range map[string]func([]state.ObjID) []decision.Option{"native": native, "rebuilt": rebuilt} {
				d := &decision.Decision{Seq: 1, Kind: decision.KChoose, Source: spell, Max: 8, Options: opts(tc.offer)}
				in := Decide(b, d, rng(1))
				var got []state.ObjID
				for _, i := range in.Choices {
					got = append(got, d.Options[i].Obj)
				}
				if !slices.Equal(got, tc.want) {
					t.Errorf("%s: tapped %v, want %v", shape, got, tc.want)
				}
			}
		})
	}
}

// Improvise pays only generic, so a coloured pip the artifacts can't touch leaves
// nothing they can finish and the ask falls back to the first offer.
func TestCastHelpersImprovisePaysOnlyGeneric(t *testing.T) {
	var b Board
	b.Pool = state.Mana{1, 0, 0, 0, 0, 0}
	b.Stack = []StackEntry{{ID: 9, IsSpell: true, ManaCost: "2 W"}}
	b.Cards.Set(1, Card{OnBattlefield: true})
	b.Cards.Set(2, Card{OnBattlefield: true})
	b.Cards.Set(3, Card{OnBattlefield: true})
	opts := []decision.Option{
		{Index: 0, Kind: "improvise_generic", Obj: 1},
		{Index: 1, Kind: "improvise_generic", Obj: 2},
		{Index: 2, Kind: "improvise_generic", Obj: 3},
	}
	d := &decision.Decision{Seq: 1, Kind: decision.KChoose, Source: 9, Max: 2, Options: opts}
	if in := Decide(b, d, rng(1)); !slices.Equal(in.Choices, []int{0, 1}) {
		t.Fatalf("choices %v, want the two helpers its generic needs", in.Choices)
	}
	b.Pool = state.Mana{}
	if in := Decide(b, d, rng(1)); !slices.Equal(in.Choices, []int{0}) {
		t.Fatalf("choices %v, want the first offer when no helpers can finish it", in.Choices)
	}
}
