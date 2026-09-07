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
