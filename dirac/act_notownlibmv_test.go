package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNotOwnLibMVReadsTheOtherPlayersReveal: ExNotOwnLibMV (Parker Luck's
// Count$ValidLibrary Card.IsRemembered+!RememberedPlayerOwn$CardManaCost)
// inside a RepeatEach over players sums the mana values of the remembered
// library cards the repeated player does not own: the other player's
// revealed card, 0 when that player revealed nothing, and 0 outside a loop.
func TestNotOwnLibMVReadsTheOtherPlayersReveal(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}})
	s := sc.s
	// two distinct mana values on the two reveals
	sc.k.cards[id["Plains"]].total = 2
	sc.k.cards[id["Forest"]].total = 5
	x := &sc.k.t.Exprs
	*x = append(*x, kop.Expr{Kind: kop.ExNotOwnLibMV})
	v := kop.ValueRef{K: kop.VExpr, N: int16(len(*x) - 1)}

	top := func(p uint8, card CardID) Obj {
		o := s.pl[p].hand[0]
		s.detach(o)
		s.objs[o].card = card
		s.objs[o].zone = kop.ZoneLibrary
		s.pl[p].libBottom(o)
		return o
	}
	o0, o1 := top(0, id["Plains"]), top(1, id["Forest"])
	if s.objs[o0].owner() != 0 || s.objs[o1].owner() != 1 {
		t.Fatalf("owners %d/%d, want 0/1", s.objs[o0].owner(), s.objs[o1].owner())
	}
	if m0, m1 := s.objMV(sc.k, s.objs[o0].card, o0), s.objMV(sc.k, s.objs[o1].card, o1); m0 != 2 || m1 != 5 {
		t.Fatalf("reveal mana values %d/%d, want 2/5 (they must differ)", m0, m1)
	}
	s.remember(o0)
	s.remember(o1)
	eval := func() int { return s.evalV(sc.k, v, evalCtx{}) }

	if got := eval(); got != 0 {
		t.Errorf("outside a RepeatEach: %d, want 0", got)
	}
	s.g.repEnd = 1 // a RepeatEach over players is in progress
	s.g.eachPl = 0
	if got := eval(); got != 5 {
		t.Errorf("repeated player 0: %d, want the opponent's reveal (5)", got)
	}
	s.g.eachPl = 1
	if got := eval(); got != 2 {
		t.Errorf("repeated player 1: %d, want the controller's reveal (2)", got)
	}
	// the opponent revealed nothing (empty library at trigger time)
	s.g.nrem = 1
	s.g.eachPl = 0
	if got := eval(); got != 0 {
		t.Errorf("other player revealed nothing: %d, want 0", got)
	}
}
