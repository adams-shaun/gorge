package kgen

import "slices"

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

// oppCountBound returns a target bound of the opponents' count
// (PlayerCountOpponents$Amount) unchanged: it is a live count the kernel
// evaluates when the slot is announced (build_ab.go's TgfBoundsV arm,
// tgtBoundReads), or a for-each-player slot whose maximum forEachOpp sets to
// the opponents a game seats (lower_targeteachopponent.go). No seat count is
// folded here.
func (lc *lowerCtx) oppCountBound(v lValue) lValue {
	return v
}

// oppCountMax is oppCountBound on a target maximum, which a OneEach slot
// keeps as it is: forEachOpp sets its bound (lower_targeteachopponent.go).
func (lc *lowerCtx) oppCountMax(ps *params, v lValue) lValue {
	if lc.oppOneEach(ps) {
		return v
	}
	return lc.oppCountBound(v)
}
