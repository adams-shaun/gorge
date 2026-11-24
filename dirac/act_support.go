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

// supportOK: the creature at sl may be supported by instruction in. The
// "other" excludes the resolving source only while it is still the object
// the ability came from: a source that left the battlefield and returned
// since (Nephalia Smuggler flickering a Saddleback Lagac copy under its own
// support trigger) is a new object (CR 400.7), so it may be supported --
// gorge's Other with SourceReturned, and objRef's RefSelf version guard.
func (s *State) supportOK(k *prep, in *kop.Instr, sl Slot) bool {
	if !s.isCreature(k, sl) {
		return false
	}
	o := s.perms[sl].obj
	// "Other" excludes the ability's source incarnation only: a source that
	// left and returned (a blink in response to its own trigger) is a new
	// object, so the creature on the battlefield now is another one
	// (CR 400.7, 701.41a); gorge's +Other reads the same incarnation.
	return in.C&0x100 == 0 || o != s.res.e.obj || s.objs[o].ver != s.res.e.srcVer
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
	if in.C&0x1000 != 0 {
		// An instant or sorcery's Support targets are announced while
		// casting (CR 701.41a with 601.2c; kgen lowers a 0..N target slot
		// and gorge's supportTargetSA asks through the cast target flow).
		// Apply one counter to each announced target; no resolution pick.
		s.supportAnnounced(k, in)
		return true
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
	// Support's picks are one counter-adding action: gorge's putCounterSupport
	// runs inside effPutCounter's deferred action bracket, so every pick merges
	// into one CounterAddedAll trigger. The ask spans resumptions and the
	// marker lives in genState; OpSupport is a top-level counter primitive (no
	// counter loop calls it), so the marker always opens at 0.
	s.ctrAllBatchOpen()
	return false
}

// supportAnnounced applies OpSupport's cast-time-target form (C&0x1000): one
// counter of C&0xFF on the CURRENT target of Obj's slot. A slot holding
// several announced targets runs this instruction once per target (the
// generic each-target loop, interp_gen.go), exactly as the ordinary
// OpPutCounter does; the loop also brackets the placements as one counter
// action. objRef returns NoSlot for a target that is no longer a legal
// battlefield object (CR 608.2b).
func (s *State) supportAnnounced(k *prep, in *kop.Instr) {
	n := int(s.value(k, in.A))
	if n <= 0 {
		return
	}
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	old := s.ctrAllBatchOpen() // one action across the targets (trig_counteraddedall.go)
	s.bolsterPut(k, sl, int(in.C&0xFF), 1)
	s.ctrAllBatchClose(old)
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
	s.ctrAllBatchClose(0) // the support action's picks end here (trig_counteraddedall.go)
	r.ask = raNone
	s.continueRes(k, 0)
}
