package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// countHand is ExCountHand: the cards in either hand matching x.F (gorge
// Count$ValidHand <spec>; Land Grant's no-lands-in-hand gate).
//
//go:noinline
func (s *State) countHand(k *prep, x *kop.Expr, ctx evalCtx) int {
	v := 0
	for pp := uint8(0); pp < MaxSeats; pp++ {
		pl := &s.pl[pp]
		for j := 0; j < int(pl.handN); j++ {
			o := pl.hand[j]
			if s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
				v++
			}
		}
	}
	return v
}
