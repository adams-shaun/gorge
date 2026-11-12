package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHandCards: the card read of a hand skips a token waiting in it (CR
// 111.6, 704.5d; gorge's zoneCountFold.visit skips an Ephemeral object).
func TestHandCards(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 5}})
	i := int16(len(sc.k.t.Exprs))
	sc.k.t.Exprs = append(sc.k.t.Exprs,
		kop.Expr{Kind: kop.ExHandSizeOf, N: 1, Ref: kop.RefController}, // i
		kop.Expr{Kind: kop.ExHandSizeOf, N: 0, Ref: kop.RefController}) // i+1
	card, raw := kop.ValueRef{K: kop.VExpr, N: i}, kop.ValueRef{K: kop.VExpr, N: i + 1}
	// Two cards in the hand beside a token that bounced into it and waits
	// there for the next state-based check: it is no card (CR 111.6).
	tok := sc.hand(id["Plains"], 0)
	sc.s.objs[tok].of |= ofToken
	if n := sc.s.handCards(0); n != 0 {
		t.Fatalf("the token alone in the hand: handCards=%d, want 0", n)
	}
	if n := int(sc.s.pl[0].handN); n != 1 {
		t.Fatalf("handN with the waiting token: %d, want 1", n)
	}
	if got := sc.s.evalV(sc.k, card, evalCtx{}); got != 0 {
		t.Fatalf("ExHandSizeOf N 1 over the token alone: %d, want 0", got)
	}
	if got := sc.s.evalV(sc.k, raw, evalCtx{}); got != 1 {
		t.Fatalf("ExHandSizeOf N 0 over the token alone: %d, want 1", got)
	}
	sc.hand(id["Plains"], 0)
	sc.hand(id["Plains"], 0)
	if n := sc.s.handCards(0); n != 2 {
		t.Fatalf("handCards with two cards and the token: %d, want 2", n)
	}
	if n := int(sc.s.pl[0].handN); n != 3 {
		t.Fatalf("handN with two cards and the token: %d, want 3", n)
	}
	// N 1 lowers Count$ValidHand Card.YouOwn (Stingerback Terror) to the
	// card read; N 0 stays the raw Count$CardsInYourHand zone length.
	if got := sc.s.evalV(sc.k, card, evalCtx{}); got != 2 {
		t.Fatalf("ExHandSizeOf N 1: %d, want 2", got)
	}
	if got := sc.s.evalV(sc.k, raw, evalCtx{}); got != 3 {
		t.Fatalf("ExHandSizeOf N 0: %d, want 3", got)
	}
}
