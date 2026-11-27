package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// castFreeFillPlayerZone offers the cards in player p's hand or graveyard, in
// zone order: a free cast whose population is the ability's player target
// (Valid$ Card.TargetedPlayerCtrl ValidZone$ Hand/Graveyard; Mindclaw
// Shaman's revealed hand, Sorcerous Squall's instant or sorcery milled from
// that player's graveyard). gorge effPlay walks every seat's zone for that
// filter and keeps the targeted player's cards, which are exactly p's zone in
// order (the target seat only, matching Card.TargetedPlayerCtrl -- not the
// all-seat graveyard walk castFreeFillGen keeps for filter populations).
// Only these two zones read it. add reports false once the offer list is
// full.
func (s *State) castFreeFillPlayerZone(p uint8, zone kop.Zone, add func(Obj) bool) {
	if p >= s.nSeats() || s.out(p) {
		return
	}
	pl := &s.pl[p]
	switch zone {
	case kop.ZoneHand:
		for i := 0; i < int(pl.handN); i++ {
			if !add(pl.hand[i]) {
				return
			}
		}
	case kop.ZoneGraveyard:
		for i := 0; i < int(pl.graveN); i++ {
			if !add(pl.grave[i]) {
				return
			}
		}
	}
}
