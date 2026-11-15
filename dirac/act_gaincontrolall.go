package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gainControlAllHits lists OpGainControl's AllValid$ subjects (Obj RefNone,
// filter f): every battlefield permanent f admits for the resolving entry,
// in object order, all listed before any control moves -- gorge
// effGainControl collects them in one g.Objs walk and then hands each over
// in that order, and each move rejoins the end of the new controller's
// battlefield list (events changeControl), so the order is observable.
func (s *State) gainControlAllHits(k *prep, f kop.FilterID, list *[MaxPerm]Slot) int {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPerm(k, f, sl, e.ctrl, src) {
			continue
		}
		o := s.perms[sl].obj
		j := n
		for ; j > 0 && s.perms[list[j-1]].obj > o; j-- {
			list[j] = list[j-1]
		}
		list[j] = sl
		n++
	}
	return n
}
