package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// annihilateAll applies CR 704.5q to the permanents of m.
//
//go:noinline
func (s *State) annihilateAll(m slotMask) {
	for ; m.any(); m = m.rest() {
		s.annihilateCtrs(m.first())
	}
}

// annihilateCtrs removes as many +1/+1 and -1/-1 counters from sl as it has
// of the rarer kind. sba's walk marks only a permanent with both.
func (s *State) annihilateCtrs(sl Slot) {
	p := &s.perms[sl]
	n := int(p.p1)
	if m1 := p.ctrLow(int(kop.CtrM1M1)); m1 < n {
		n = m1
	}
	p.seeCtr(s)
	p.subCtrN(s, int(kop.CtrP1P1), n)
	p.subCtrN(s, int(kop.CtrM1M1), n)
	s.touchSl(sl)
}
