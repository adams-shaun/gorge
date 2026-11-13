package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// --- OpSupport's remove form ---
//
// gorge removeCounterChoose (effects/removecounter.go): RemoveCounter with
// Choices$ elects among every battlefield permanent matching the filter
// (either controller, no Other rule); each chosen one loses min(1, count)
// counters of the kind. ChoiceNum$ N is an exact N (min = max = N), the
// ChoiceOptional$ form any number (min 0, max the eligible count). NO ASK
// when nothing is eligible or when an exact N covers every eligible one (the
// strict-supersets rule: all of them lose a counter). Otherwise one
// "counter_pick" KChoose, asked here one pick at a time like plain Support.
// A chosen permanent holding none of the kind is a legal pick that removes
// nothing. RememberAmount$ True adds the removed total to remCnt.
//
// Instr: A the pick count (16 for any number), C&0xFF the kind, C&0x200 this
// form, C&0x400 exact, C&0x800 RememberAmount$, F the Choices$ filter.

// DebugRmctrNoRemember: the remember bit adds nothing (mutation test).
var DebugRmctrNoRemember bool

// rmctrOptions lists the unpicked eligible permanents (seat 0's, then seat
// 1's), and Done when picks may stop early (any number); it returns the
// count of permanents listed.
func (s *State) rmctrOptions(k *prep, in *kop.Instr, d *Decision) int {
	src := s.objRef(k, kop.RefSelf)
	n := 0
	for p := uint8(0); p < MaxSeats; p++ {
		for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
			sl := m.first()
			o := uint8(s.perms[sl].obj)
			if s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) && !s.prolifPicked(o) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
				n++
			}
		}
	}
	if in.C&0x400 == 0 || n == 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return n
}

// rmctrTake removes one counter of the kind from the permanent at sl (the
// OpRemoveCounterRem removal); a permanent holding none loses nothing.
func (s *State) rmctrTake(in *kop.Instr, sl Slot) {
	kind := int(in.C & 0xFF)
	if s.perms[sl].ctrN(s, kind) < 1 {
		return
	}
	s.touchSl(sl)
	p := &s.perms[sl]
	p.seeCtr(s)
	p.subCtrN(s, kind, 1)
	if in.C&0x800 != 0 && !DebugRmctrNoRemember {
		s.g.remCnt++
	}
}

// rmctrStart is OpSupport's remove form; false: suspended on the pick.
func (s *State) rmctrStart(k *prep, in *kop.Instr) bool {
	r := &s.res
	r.peekN = 0
	var d Decision
	eligible := s.rmctrOptions(k, in, &d)
	if eligible == 0 {
		return true
	}
	n := int(in.A.N)
	if in.C&0x400 != 0 && eligible <= n { // the forced exact set: no ask
		var ord [MaxPerm]Slot
		cnt := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:cnt] {
			s.rmctrTake(in, sl)
		}
		return true
	}
	r.left = uint8(min(n, eligible))
	r.ask, r.askP = raGenSupport, r.e.ctrl
	return false
}

// answerRmctr applies one pick (or Done) of the remove form's ask.
func (s *State) answerRmctr(k *prep, in *kop.Instr, a Action) {
	r := &s.res
	if a.Kind() != AkDone {
		o := a.Source()
		if int(r.peekN) >= prolifMaxPicks {
			s.capFull(StallProlif)
		} else {
			if int(r.peekN) < len(r.arr) {
				r.arr[r.peekN] = Obj(o)
			} else {
				r.peek[int(r.peekN)-len(r.arr)] = Obj(o)
			}
			r.peekN++
		}
		if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
			s.rmctrTake(in, ob.slot)
		}
		r.left--
		var d Decision
		if r.left > 0 && s.rmctrOptions(k, in, &d) > 0 {
			return // more to pick
		}
	}
	r.ask = raNone
	s.continueRes(k, 0)
}
