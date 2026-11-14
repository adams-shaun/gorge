package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// castFreeFillHand offers the cards in player p's hand, in hand order: a
// free cast whose population is the ability's player target (Valid$
// Card.TargetedPlayerCtrl ValidZone$ Hand; Mindclaw Shaman's revealed
// hand). gorge effPlay walks every seat's hand for that filter and keeps
// the targeted player's cards, which are exactly p's hand in order. Only
// the hand zone reads it. add reports false once the offer list is full.
func (s *State) castFreeFillHand(p uint8, zone kop.Zone, add func(Obj) bool) {
	if zone != kop.ZoneHand || p >= s.nSeats() || s.out(p) {
		return
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if !add(pl.hand[i]) {
			return
		}
	}
}
