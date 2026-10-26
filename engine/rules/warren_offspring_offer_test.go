package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// warrenLandsEngine deals seat 0 Warren Warleader plus Plains and Forests and
// advances to Main1; the opponent's deck is all Forests.
func warrenLandsEngine(t *testing.T) *Engine {
	t.Helper()
	reg := searchTestRegistry(t)
	warren := searchCorpusCard(t, reg, "Warren Warleader")
	plains := searchCorpusCard(t, reg, "Plains")
	forest := searchCorpusCard(t, reg, "Forest")
	deck := []*cards.Card{warren}
	for i := 0; i < 8; i++ {
		deck = append(deck, plains, forest)
	}
	for len(deck) < 40 {
		deck = append(deck, forest)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = forest
	}
	e := New(seatZeroStart(Config{Seed: 4242, Names: []string{"warren", "opp"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens}))
	e.Advance()
	toMain1(t, e)
	return e
}

// warrenPutLand moves one named basic from hand/library to the battlefield.
func warrenPutLand(t *testing.T, e *Engine, name string) {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
				return
			}
		}
	}
	t.Fatalf("no %s for seat 0", name)
}

// warrenModes reports which cast modes of hero the potential-action walk
// offers from the untapped lands (a land-fed seat never floats mana before
// the offer, so the pool-priced walk offers nothing).
func warrenModes(t *testing.T, e *Engine, hero state.ObjID) (plain, offspring bool) {
	t.Helper()
	e.pending = nil
	e.askPriority(0)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != hero || pp.Reason == "insufficient" {
			continue
		}
		switch pp.Action.Mode {
		case "":
			plain = true
		case "offspring":
			offspring = true
		}
	}
	return plain, offspring
}

// TestWarrenWarleaderOffersOffspring pins that Warren Warleader's printed
// K:Offspring:2, which sits beside an AttackersDeclared Charm trigger, still
// yields the optional additional-cost offer (CR 702.175a) from real lands.
func TestWarrenWarleaderOffersOffspring(t *testing.T) {
	t.Parallel()
	e := warrenLandsEngine(t)
	hero := searchMoveByName(t, e, "Warren Warleader", state.ZHand)
	if !e.HasKeyword(hero, "Offspring") {
		t.Fatal("precondition: Warren Warleader must have Offspring")
	}
	// Exactly {2}{W}{W} untapped: only the plain cast.
	for i := 0; i < 2; i++ {
		warrenPutLand(t, e, "Plains")
		warrenPutLand(t, e, "Forest")
	}
	plain, off := warrenModes(t, e, hero)
	if !plain {
		t.Fatal("plain cast not offered with {2}{W}{W}")
	}
	if off {
		t.Fatal("offspring cast offered with only {2}{W}{W}")
	}
	// Six lands ({W}{W} plus four generic): base + {2} is affordable.
	warrenPutLand(t, e, "Forest")
	warrenPutLand(t, e, "Forest")
	plain, off = warrenModes(t, e, hero)
	if !plain || !off {
		t.Fatalf("with six lands plain=%v offspring=%v, want both", plain, off)
	}
}
