package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Katilda, Dawnhart Prime grants Humans "{T}: Add one mana of any of this
// creature's colors" (ManaReflected Valid$ Defined.Self, ReflectProperty$ Is).
// The payment census must price each granted ability as the Human's colours,
// not the {C} stand-in the shared census lists for a blank Produced$.
func katildaBoard(t *testing.T, humans ...string) (*Engine, []state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := []*cards.Card{lookup(t, reg, "Katilda, Dawnhart Prime"), lookup(t, reg, "Savannah Lions")}
	for _, h := range humans {
		deck = append(deck, lookup(t, reg, h))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, "Katilda, Dawnhart Prime"))
	var hs []state.ObjID
	for _, h := range humans {
		id := findAndMoveToHand(t, e, 0, h)
		moveToBattlefield(t, e, id)
		hs = append(hs, id)
	}
	lions := findAndMoveToHand(t, e, 0, "Savannah Lions")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	return e, hs, lions
}

// katildaPotentialCast reports whether the priority decision lists cast id as
// a potential cast (castable once its mana abilities are activated).
func katildaPotentialCast(e *Engine, id state.ObjID) bool {
	e.EnsurePaymentActions()
	return slices.Contains(e.Pending().PotentialCasts, id)
}

func TestKatildaGrantedReflectedManaPaysColour(t *testing.T) {
	e, hs, lions := katildaBoard(t, "Elite Vanguard")
	katilda := e.G.Zone(state.ZBattlefield, 0)[0]
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != 2 {
		t.Fatalf("precondition: want Katilda + Elite Vanguard only on the battlefield, got %d permanents", n)
	}
	if !katildaPotentialCast(e, lions) {
		t.Fatalf("Savannah Lions is not a potential cast; a granted Defined.Self reflected ability must pay its Human's colour")
	}
	var tapsHuman bool
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != lions {
			continue
		}
		for _, step := range pp.Script {
			for _, pick := range step.Picks {
				tapsHuman = tapsHuman || (pick.Kind == "mana" && pick.ManaSymbol == "W" && (pick.Obj == hs[0] || pick.Obj == katilda))
			}
		}
	}
	if !tapsHuman {
		t.Fatalf("the potential plan for Lions does not tap a Human for W")
	}

	// A colourless Human reflects nothing: with Katilda tapped it is the only
	// Human able to tap, and Lions is not offered.
	e2, _, lions2 := katildaBoard(t)
	for _, id := range e2.G.Zone(state.ZBattlefield, 0) {
		e2.G.Obj(id).Tapped = true
	}
	blank := onBoardCard(t, e2, 0, card(t, "Name:Blank Human\nTypes:Creature Human\nPT:1/1\nOracle:x\n"))
	e2.G.Obj(blank).SummonSick = false
	e2.staticEpoch, e2.activeEpoch = -1, -1
	if c := e2.Colors(blank); c != "" || e2.G.Obj(blank).Tapped || e2.G.Obj(blank).SummonSick {
		t.Fatalf("precondition: want an untapped, unsick colourless Human, got colours %q", c)
	}
	if katildaPotentialCast(e2, lions2) {
		t.Fatalf("Lions is a potential cast with only a colourless Human untapped")
	}
}
