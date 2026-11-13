package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An emblem that remembers a set (kop.Emblem.CaptureRem) hands the set to its
// trigger as the many-object carriers of a delayed program (trig_delayedmany.go
// dpManyCarrier: trigObj, trigAmt, x, trigMV; skwMany stamps the versions as
// the trigger goes on the stack), so its body reads the objects the emblem
// remembers at the moment the trigger fires.

// embRemCarry fills pt's carriers with the objects player p's emblem i
// remembers, packed in slot order (an empty slot, a token that left, is
// skipped); an empty set leaves every carrier empty.
func (s *State) embRemCarry(p uint8, i int, pt *pendingTrig) {
	pt.trigObj, pt.trigAmt, pt.x, pt.trigMV = NoObj, 0, 0, 0
	pt.skw |= skwMany
	n := 0
	for j := 0; j < embRemSlots; j++ {
		v := s.embRemGet(p, i, j)
		if v == 0 {
			continue
		}
		o := Obj(v - 1)
		switch n {
		case 0:
			pt.trigObj, pt.remVer = o, objWord(s.objs[o].ver)
		case 1:
			pt.trigAmt = int16(o) + 1
		case 2:
			pt.x = objWord(o) + 1
		default:
			pt.trigMV = objWord(o) + 1
		}
		n++
	}
}

// exileEmbRem is kop.ExtExileEmbRem: each object the resolving entry names
// that is still the object it remembered, on the battlefield, is exiled.
//
//go:noinline
func (s *State) exileEmbRem(k *prep) {
	e := &s.res.e
	for i := 0; i < embRemSlots; i++ {
		o, ver := e.dpManyCarrier(i)
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield || s.objs[o].ver != ver {
			continue
		}
		s.leave(k, s.objs[o].slot, kop.ZoneExile, false)
	}
}
