package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- support (CR 701.41a) ---
//
// gorge putCounterSupport: Support$ N is ONE counter (CounterType$, default
// P1P1) on each of up to N creatures (Creature.+Other for a permanent's
// ability, Creature for an instant or sorcery), any controller, picked in
// one KChoose Min 0 Max min(N, eligible) "counter_pick" by the resolving
// controller; nothing when N <= 0 or nothing is eligible. The kernel asks
// one pick at a time (ChGenBolster plus Done, the proliferate shape),
// applies each as it is answered (gorge applies the answer at its commit,
// in answer order) and closes the pick at the max-th one; res.arr/res.peek
// hold the picks (a seventeenth stalls, StallProlif) and res.left the
// picks still allowed.

// supportOK: the creature at sl may be supported by instruction in.
func (s *State) supportOK(k *prep, in *kop.Instr, sl Slot) bool {
	if !s.isCreature(k, sl) {
		return false
	}
	return in.C&0x100 == 0 || s.perms[sl].obj != s.res.e.obj
}

// supportOptions lists the unpicked eligible creatures (seat 0's, then
// seat 1's) and Done.
func (s *State) supportOptions(k *prep, in *kop.Instr, d *Decision) {
	if in.C&0x200 != 0 {
		s.rmctrOptions(k, in, d) // ctr_removechoices.go
		return
	}
	for p := uint8(0); p < MaxSeats; p++ {
		for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
			sl := m.first()
			o := s.perms[sl].obj
			if s.supportOK(k, in, sl) && !s.prolifPicked(TCode(o)) {
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// support is OpSupport; false: suspended on the pick.
func (s *State) support(k *prep, in *kop.Instr) bool {
	if in.C&0x200 != 0 {
		return s.rmctrStart(k, in) // ctr_removechoices.go
	}
	n := s.value(k, in.A)
	if n <= 0 {
		return true
	}
	r := &s.res
	r.peekN = 0
	var d Decision
	s.supportOptions(k, in, &d)
	if d.N <= 1 {
		return true // nothing eligible
	}
	r.left = uint8(min(n, 255))
	r.ask, r.askP = raGenSupport, r.e.ctrl
	return false
}

// answerSupport applies one pick (or Done) of the support ask.
func (s *State) answerSupport(k *prep, a Action) {
	r := &s.res
	in := &s.prog(k)[r.pc]
	if in.C&0x200 != 0 {
		s.answerRmctr(k, in, a)
		return
	}
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
			s.bolsterPut(k, ob.slot, int(in.C&0xFF), 1)
		}
		r.left--
		var d Decision
		s.supportOptions(k, in, &d)
		if r.left > 0 && d.N > 1 {
			return // more to pick
		}
	}
	r.ask = raNone
	s.continueRes(k, 0)
}
