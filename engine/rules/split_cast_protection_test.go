package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// armedAgainst deals seat 0 Armed // Dangerous in hand with two red mana and
// puts one creature named target on seat 1's battlefield.
func armedAgainst(t *testing.T, target string) (*Engine, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	armed := searchCorpusCard(t, reg, "Armed")
	tgt := searchCorpusCard(t, reg, target)
	island := searchCorpusCard(t, reg, "Island")
	deck := []*cards.Card{armed}
	for len(deck) < 40 {
		deck = append(deck, island)
	}
	opp := []*cards.Card{tgt}
	for len(opp) < 40 {
		opp = append(opp, island)
	}
	cfg := seatZeroStart(Config{Seed: 8425, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	splitMoveFromLibrary(t, e, 1, target)
	id := searchMoveByName(t, e, "Armed", state.ZHand)
	addMana(t, e, 0, "RR")
	return e, id
}

// CR 709.3b with CR 702.16b: a split card cast as one half is only that half
// (Armed is red), though in hand it has both halves' colours (CR 709.4:
// Armed // Dangerous is red and green). Blightbeetle's protection from green
// therefore does not keep Armed from targeting it, and the offer census must
// say so (dr-mbc-aabb pair 91: the kernel offered the cast, gorge did not).
func TestSplitHalfCastTargetsIgnoreTheOtherHalfsColour(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"Grizzly Bears", "Blightbeetle"} {
		e, id := armedAgainst(t, target)
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZHand {
			t.Fatalf("%s: precondition: Armed not in hand: %+v", target, o)
		}
		if got := e.objColors(o); got != "RG" && got != "GR" {
			t.Fatalf("%s: precondition: Armed // Dangerous in hand has colours %q, want red and green", target, got)
		}
		if len(castOptionsFor(e, id)) == 0 {
			t.Errorf("%s: Armed is castable on it (the spell is red only) but no cast was offered", target)
		}
	}
}
