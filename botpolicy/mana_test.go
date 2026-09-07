package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
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
// production the projection now reports for an Indeterminate (non-literal
// Amount$) source, which might add zero mana. The pre-fix tap gate ranked
// by fewest distinct colours and so tapped the zero-flex empty source first,
// trusting mana that never arrives; the tiering puts a demonstrable producer
// first whatever its colour, so a tap always buys something.
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
