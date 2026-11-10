package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// selfDepartCtr is the last known count of counter kind c (kop.CtrAll: every
// kind but a Class's level) on ability e's source o, which left the battlefield
// once since the ability was created, read from the departure snapshot. It is
// the counters read selfCountersLKI cannot make once the departed permanent's
// slot was reused: Jadzia Dax's first-main trigger, still on the stack when
// Impossible Choice destroyed her, adds {G} for her last counters, and the two
// Officer tokens that came in after took her slot (CR 608.2h). ok is false when
// the read is live or no snapshot exists.
func (s *State) selfDepartCtr(e *stackEntry, o Obj, c kop.CounterKind) (n int, ok bool) {
	if e == nil || e.kind == seSpell || o == NoObj || o != e.obj {
		return 0, false
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield || ob.ver != e.srcVer+1 {
		return 0, false
	}
	i := s.srcDepartIdx(e)
	if i == 0 {
		return 0, false
	}
	if c != kop.CtrAll {
		return s.lkiCounter(i, c), true
	}
	return s.lkiCountersAll(i), true
}
