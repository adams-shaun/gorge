package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handCards counts the cards in player p's hand. A token bounced to the hand
// waits there for the next state-based check (CR 704.5d) and is no card (CR
// 111.6), so it does not count; gorge's zoneCountFold.visit skips an
// Ephemeral object the same way (Stingerback Terror). The raw zone length is
// handN (Count$CardsInYourHand).
func (s *State) handCards(p uint8) int {
	n := 0
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if s.objs[pl.hand[i]].of&(ofToken|ofCopy) == 0 {
			n++
		}
	}
	return n
}

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
			if s.objs[o].of&(ofToken|ofCopy) != 0 {
				continue // a token waiting in the hand is no card (CR 111.6, 704.5d)
			}
			if s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
				v++
			}
		}
	}
	return v
}
