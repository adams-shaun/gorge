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
	if t == NoTarget || t >= PlayerTarget0 {
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
