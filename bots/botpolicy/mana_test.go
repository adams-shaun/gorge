package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func prod(col int, n int32) cards.ManaProduction {
	var mp cards.ManaProduction
	if n <= 0 {
		n = 1
	}
	mp.Colour[col] = n
	return mp
}

func dual(w, u int32) cards.ManaProduction {
	var mp cards.ManaProduction
	mp.Colour[0] = w
	mp.Colour[1] = u
	return mp
}

func TestChooseTapPicksNeededColourLeastFlexible(t *testing.T) {
	b := priorityCards(map[state.ObjID]Card{
		1:  {CMC: 1, ManaCost: "U", Castable: true},
		10: {Produces: prod(state.MU, 1)},
		11: {Produces: dual(1, 1)},
	})
	got, d := castDecision(b, []decision.Option{
		activate(0, 11),
		activate(1, 10),
	})
	if d.Options[got].Obj != 10 {
		t.Fatalf("tap = obj %d (option %d), want mono-colour Island (%d) over dual", d.Options[got].Obj, got, 10)
	}
}

func TestChooseTapAvoidsWrongColour(t *testing.T) {
	b := priorityCards(map[state.ObjID]Card{
		1:  {CMC: 1, ManaCost: "U", Castable: true},
		10: {Produces: prod(state.MR, 1)},
		11: {Produces: prod(state.MU, 1)},
	})
	got, d := castDecision(b, []decision.Option{
		activate(0, 10),
		activate(1, 11),
	})
	if d.Options[got].Obj != 11 {
		t.Fatalf("tap = obj %d (option %d), want blue source (%d), not first option", d.Options[got].Obj, got, 11)
	}
}

func TestChooseTapFallsBackWhenNoColourMatch(t *testing.T) {
	b := priorityCards(map[state.ObjID]Card{
		1:  {CMC: 2, ManaCost: "2", Castable: true},
		10: {Produces: prod(state.MW, 1)},
		11: {Produces: dual(1, 1)},
	})
	got, d := castDecision(b, []decision.Option{
		activate(0, 11),
		activate(1, 10),
	})
	if d.Options[got].Obj != 10 {
		t.Fatalf("tap = obj %d (option %d), want least-flexible mono source (%d)", d.Options[got].Obj, got, 10)
	}
}

func activate(idx int, id state.ObjID) decision.Option {
	return decision.Option{Index: idx, Kind: "activate", Obj: id}
}

// TestChooseTapPrefersDemonstrableOverEmpty is the bl1 tiering on the policy
// side of the production-honesty fix: a source that demonstrably produces
// mana (here one red), even a colour the intended card does not need, is
// tapped before a source that demonstrably produces nothing -- the empty
// production the projection reports for an Indeterminate (non-literal
// Amount$) source, which might add zero mana. A tap gate ranking by fewest
// distinct colours would tap the zero-flex empty source first, trusting mana
// that never arrives; the tiering puts a demonstrable producer first whatever
// its colour, so a tap always buys something.
func TestChooseTapPrefersDemonstrableOverEmpty(t *testing.T) {
	b := priorityCards(map[state.ObjID]Card{
		1:  {CMC: 1, ManaCost: "2", Castable: true}, // a generic one-cost spell
		10: {Produces: prod(state.MR, 1)},           // a real producer (red)
		11: {},                                      // empty production (an Indeterminate source claims none)
	})
	got, d := castDecision(b, []decision.Option{
		activate(0, 10), // the demonstrable producer, listed first
		activate(1, 11), // the empty-production source, listed second
	})
	if d.Options[got].Obj != 10 {
		t.Fatalf("tap = obj %d (option %d), want the demonstrable producer (obj 10), not the empty source", d.Options[got].Obj, got)
	}
}

// manaColourAsk is a source's "which mana" ask over the given options.
func manaColourAsk(opts ...decision.Option) *decision.Decision {
	for i := range opts {
		opts[i].Index, opts[i].Kind = i, "mana"
	}
	return &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts}
}

// TestChooseManaColourFloatsWhatTheCardNeeds: a dual tapped toward a blue spell
// adds blue, not its first colour, whichever way the engine spelled the colour.
func TestChooseManaColourFloatsWhatTheCardNeeds(t *testing.T) {
	blue := priorityCards(map[state.ObjID]Card{1: {CMC: 1, ManaCost: "U", Castable: true}})
	for _, tc := range []struct {
		name string
		b    Board
		d    *decision.Decision
		want int
	}{
		{"flattened colours", blue, manaColourAsk(decision.Option{ManaSymbol: "W"}, decision.Option{ManaSymbol: "U"}), 1},
		{"one ability per colour", blue, manaColourAsk(decision.Option{Label: "Add W"}, decision.Option{Label: "Add U"}), 1},
		{"a costed ability", blue, manaColourAsk(decision.Option{Label: "Pay 1: Add W", ManaSymbol: "W"}, decision.Option{Label: "Pay 1: Add U", ManaSymbol: "U"}), -1},
		{"colourless and a multi-pip ability name no colour", blue, manaColourAsk(decision.Option{Label: "Add C"}, decision.Option{Label: "Add UU"}), -1},
		{"nothing needs either colour", priorityCards(map[state.ObjID]Card{1: {CMC: 1, ManaCost: "1", Castable: true}}),
			manaColourAsk(decision.Option{Label: "Add W"}, decision.Option{Label: "Add U"}), -1},
		{"an announced payment window", blue, func() *decision.Decision {
			d := manaColourAsk(decision.Option{Label: "Add W"}, decision.Option{Label: "Add U"})
			d.ManaPayment = &decision.ManaPaymentWindow{}
			return d
		}(), -1},
		{"a payment window's ask", blue, func() *decision.Decision {
			d := manaColourAsk(decision.Option{Label: "Add W"}, decision.Option{Label: "Add U"})
			d.ResumeKind = decision.ResumeManaPayment
			return d
		}(), -1},
	} {
		if got := tc.b.chooseManaColour(tc.d); got != tc.want {
			t.Errorf("%s: pick %d, want %d", tc.name, got, tc.want)
		}
	}
	d := manaColourAsk(decision.Option{Label: "Add W"}, decision.Option{Label: "Add U"})
	if in := Decide(blue, d, nil); len(in.Choices) != 1 || in.Choices[0] != 1 {
		t.Errorf("Decide answered %v, want the blue option", in.Choices)
	}
}
