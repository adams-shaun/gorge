package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// srcDepartIdx is the pool index (lk's, 1-based) of the departure snapshot of
// ability e's source, 0 when there is none (zone_lkisource.go).
func (s *State) srcDepartIdx(e *stackEntry) uint8 {
	for i := range s.g.lki {
		if r := &s.g.lki[i]; r.ref != 0 && r.bits&lkiDepart != 0 && r.obj == e.obj && r.ver == e.srcVer {
			return uint8(i + 1)
		}
	}
	return 0
}

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
	l := s.lk(i)
	n = int(l.p1p1)
	for j := range l.ck {
		if l.ck[j] != 0 && kop.CounterKind(l.ck[j]) != kop.CtrLevel {
			n += int(l.cn[j])
		}
	}
	return n + s.ctrOverSum(l.obj, i), true
}
