package kgen

import "testing"

// TestHasCardsGraveyardGE7TimesTwo lowers Master's Councillors' power SVar
// (PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7/Times.2) and
// evaluates the LIR over graveyards of 6, 7 and 8 cards for each player:
// 2 power per graveyard of seven or more, as gorge's count_player.go reads it.
// gorge's zone walk counts a token waiting in the graveyard, so every
// graveyard leaf must be the token-counting form (N 1).
func TestHasCardsGraveyardGE7TimesTwo(t *testing.T) {
	lc := &lowerCtx{out: &lCard{}}
	v, err := lc.svarValue("PlayerCountPlayers$HasPropertyHasCardsInGraveyard_Card_GE7/Times.2", 1)
	if err != nil {
		t.Fatal(err)
	}
	var eval func(v lValue, you, opp int) int
	eval = func(v lValue, you, opp int) int {
		var n int
		switch v.Kind {
		case vConst:
			n = v.N
		case vCountGraveyard:
			if v.N != 1 {
				t.Errorf("graveyard leaf %s does not count tokens", v)
			}
			if v.F == nil || len(v.F.Clauses) != 1 {
				t.Fatalf("graveyard leaf filter %v", v.F)
			}
			switch v.F.Clauses[0].Owner {
			case whoYou:
				n = you
			case whoOpp:
				n = opp
			default:
				t.Fatalf("graveyard leaf owner %v", v.F.Clauses[0].Owner)
			}
		case vSum:
			for _, s := range v.Sub {
				n += eval(s, you, opp)
			}
		case vCompare:
			a := eval(v.Sub[0], you, opp)
			arm := 1
			if a >= v.N && v.Op == opGE {
				arm = 0
			}
			n = eval(v.Sub[1].Sub[arm], you, opp)
		default:
			t.Fatalf("unexpected kind %s", v)
		}
		if v.Mul != 0 {
			n *= v.Mul
		}
		if v.Div != 0 {
			n /= v.Div
		}
		n += v.Add
		if v.Neg {
			n = -n
		}
		return n
	}
	for _, you := range []int{6, 7, 8} {
		for _, opp := range []int{6, 7, 8} {
			want := 0
			if you >= 7 {
				want += 2
			}
			if opp >= 7 {
				want += 2
			}
			if got := eval(v, you, opp); got != want {
				t.Errorf("graveyards you=%d opp=%d: power %d, want %d (%s)", you, opp, got, want, v)
			}
		}
	}
}
