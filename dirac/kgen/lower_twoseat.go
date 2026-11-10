package kgen

import (
	"slices"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// twoSeat marks the card two-seat only (kop.CardDef.TwoSeat): the lowering
// just taken reads a two-player identity, such as "the opponent" for a
// player only one of several opponents can be, or an opponents count of
// one. The reduction is exact in a two-player game, which plays the card as
// before; a game of more seats refuses the table (dirac StallTwoSeat).
// reason names the reduction for a reader of the call site.
func (lc *lowerCtx) twoSeat(reason string) {
	if lc.out != nil {
		lc.out.TwoSeat = true
		if !slices.Contains(lc.out.TwoSeatWhy, reason) {
			lc.out.TwoSeatWhy = append(lc.out.TwoSeatWhy, reason)
		}
	}
}

// twoSeatOperand is the reason a card is two-seat only when no lowering
// marked it but a program operand names the one opponent (kop.TwoSeatInstr).
const twoSeatOperand = "an operand naming the one opponent (kop.TwoSeatInstr)"

// oppCountBound reads a target bound of the opponents' count
// (PlayerCountOpponents$Amount) as the one opponent: a slot's bounds are
// constants, and the count is one only in a two-player game.
func (lc *lowerCtx) oppCountBound(v lValue) lValue {
	if v.Kind != vOppExtreme || v.N != kop.OppExtAmount || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return v
	}
	lc.twoSeat("a target bound of the opponents' count read as one")
	return cst(1)
}

// livingBound keeps a target bound of the living players' count
// (PlayerCountPlayers$Amount, the opponents' count plus one) a two-player
// constant 2 on a slot whose bounds must be constants: only a plain "up to"
// slot reads the count as it is announced (build_ab.go's TgfBoundsV arm).
func (lc *lowerCtx) livingBound(t *lTarget) {
	v := t.Max
	if v.Kind != vOppExtreme || v.N != kop.OppExtAmount || v.Add != 1 || v.Mul != 0 || v.Div != 0 || v.Neg {
		return
	}
	if !t.AtRes && !t.Unique && !t.SameCtrl && !t.DiffCtrl && !t.SharedCrType && !t.DistinctName && t.Mode < 0 && t.ParentPlus1 == 0 &&
		(t.Divided == 0 || t.Divided == kop.DivMaxV || t.Divided == kop.DivValue) && t.Min.isConst() && t.Min.N == 0 {
		return
	}
	lc.twoSeat("the living players counted as two")
	t.Max = cst(2)
}
