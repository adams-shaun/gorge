package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHasCardsGraveyardGE7TimesTwo lowers Master's Councillors' power SVar
// (PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7/Times.2) and
// evaluates the LIR over graveyards of 6, 7 and 8 cards for each player:
// 2 power per graveyard of seven or more, as gorge's count_player.go reads it.
// gorge's zone walk counts a token waiting in the graveyard, so the figure
// is the token-counting graveyard length (OppExtGraveAll), and the group is
// every living player (OppExtAllPl), each compared on its own.
func TestHasCardsGraveyardGE7TimesTwo(t *testing.T) {
	lc := &lowerCtx{out: &lCard{}}
	v, err := lc.svarValue("PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7/Times.2", 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != vOppExtreme || v.N != kop.OppExtCountCmp|kop.OppExtGraveAll|kop.OppExtAllPl || v.Op != opGE || len(v.Sub) != 1 || !v.Sub[0].isConst() || v.Sub[0].N != 7 || v.Mul != 2 {
		t.Fatalf("lowered %s (N %#x, op %d), want OppExtreme(CountCmp|GraveAll|AllPl) GE 7, times 2", v, v.N, v.Op)
	}
	if lc.out.TwoSeat {
		t.Fatal("the per-member count marked the card two-seat")
	}
	for _, g := range [][]int{{6, 7}, {8, 6}, {7, 8, 6, 9}, {6, 6, 6, 6}} {
		want, got := 0, 0
		for _, n := range g {
			if n >= 7 {
				want += 2
			}
			if n >= v.Sub[0].N {
				got += v.Mul
			}
		}
		if got != want {
			t.Errorf("graveyards %v: power %d, want %d", g, got, want)
		}
	}
}

// TestHasCardsHandOpponentsPerMember: Wojek Investigator's Num$ X
// (PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GTY, Y the
// controller's hand) is the count of living opponents, each compared on its
// own, not one opponent read as all of them: a four-seat pod's two fuller
// hands investigate twice (gorge hasPropertyStateBacked).
func TestHasCardsHandOpponentsPerMember(t *testing.T) {
	lc := &lowerCtx{out: &lCard{}, face: &cards.Face{SVars: map[string]string{"Y": "Count$CardsInYourHand"}}}
	v, err := lc.svarValue("PlayerCountOpponents$HasPropertyHasCardsInHand_Card_GTY", 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != vOppExtreme || v.N != kop.OppExtCountCmp|kop.OppExtHand || v.Op != opGT || len(v.Sub) != 1 {
		t.Fatalf("lowered %s (N %#x, op %d), want OppExtreme(CountCmp|Hand) GT Y", v, v.N, v.Op)
	}
	if v.Sub[0].Kind != vHandSizeOf || v.Sub[0].Ref.Kind != rYou {
		t.Fatalf("bound %s, want the controller's hand size", v.Sub[0])
	}
	if lc.out.TwoSeat {
		t.Fatal("the per-member count marked the card two-seat")
	}
}
