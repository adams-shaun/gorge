package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sameNameT0 is PredSameNameT0 for the permanent at sl: the resolving
// ability's target 0 itself, or another permanent with its name (gorge
// Maelstrom Pulse's TargetedCard.Self or sharesNameWith Targeted; a
// face-down permanent has no name, CR 708.2).
func (s *State) sameNameT0(sl Slot) bool {
	if !s.res.active {
		return false
	}
	e := &s.res.e
	t := e.tgt[0]
	if TCode(t) == NoTarget || isPlayerOrNone(TCode(t)) {
		return false
	}
	ob := &s.objs[t]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.tver[0] {
		return false
	}
	if ob.slot == sl {
		return true
	}
	n := s.nameCardAt(ob.slot)
	return n != NoCard && n == s.nameCardAt(sl)
}

// sameNameT0Card is PredSameNameT0 for the card c at object o off the
// battlefield (gorge's "TargetedCard.Self,Card.NotDefinedTargeted+
// sharesNameWith Targeted" over a graveyard: Rat King, Verminister): the
// candidate is target 0 itself, or another card with its name. The target's
// legality is the one resolveTop took as the resolution started (res.bad):
// the sweep matches card by card as it moves them, and gorge's referent
// keeps naming the target after the sweep has moved it (an ObjID survives
// the zone change), so a card after the target in the graveyard still
// matches. A graveyard card is face up; fd only refuses a face-down
// candidate's name (CR 708.2).
func (s *State) sameNameT0Card(c CardID, o Obj, fd bool) bool {
	if !s.res.active || o == NoObj || s.res.bad&1 != 0 { // seat-ok: a resolution flag bit
		return false
	}
	t := s.res.e.tgt[0]
	if TCode(t) == NoTarget || isPlayerOrNone(TCode(t)) {
		return false
	}
	if Obj(t) == o {
		return true
	}
	return !fd && s.objs[t].card == c
}
