package kgen

// twoSeat marks the card two-seat only (kop.CardDef.TwoSeat): the lowering
// just taken reads a two-player identity, such as "the opponent" for a
// player only one of several opponents can be, or an opponents count of
// one. The reduction is exact in a two-player game, which plays the card as
// before; a game of more seats refuses the table (dirac StallTwoSeat).
// reason names the reduction for a reader of the call site.
func (lc *lowerCtx) twoSeat(reason string) {
	_ = reason
	if lc.out != nil {
		lc.out.TwoSeat = true
	}
}
