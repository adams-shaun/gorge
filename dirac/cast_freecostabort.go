package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// freeCostAbort: a free cast (a Play's WithoutManaCost$ spell chosen mid-
// resolution) whose additional cost cannot be paid (Seize the Spoils with
// no other card to discard). gorge begins the cast, pushes the card and
// aborts at the cost it cannot pay (CR 601.2b, 733.1): the card goes to the
// stack and comes back to the END of the zone it was cast from, so a card
// cast from exile behind two others sits after them (pairs 6092, 8125).
// The kernel never begins such a cast; this reproduces the reorder only.
func (s *State) freeCostAbort(o Obj) {
	ob := &s.objs[o]
	pl := &s.pl[ob.owner()]
	switch ob.zone {
	case kop.ZoneHand:
		s.touch(o)
		removeFrom(pl.hand[:], &pl.handN, o)
		pl.hand[pl.handN] = o
		pl.handN++
	case kop.ZoneGraveyard:
		s.touch(o)
		removeFrom(pl.grave[:], &pl.graveN, o)
		pl.grave[pl.graveN] = o
		pl.graveN++
	case kop.ZoneExile:
		s.touch(o)
		removeFrom(pl.exile[:], &pl.exileN, o)
		pl.exile[pl.exileN] = o
		pl.exileN++
	}
}
