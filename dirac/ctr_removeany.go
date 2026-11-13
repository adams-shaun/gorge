package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext RemoveCounterAny (gorge removeCounterAny,
// cr-removecounter-any-upto 461d8b818): "remove up to three counters from
// target permanent" (Glissa Sunslayer). The controller picks the kind of
// each counter removed, one at a time, among the kinds still present after
// the picks so far; with UpTo$ a Done option stops the election at every
// step, without it a single present kind is taken unasked. The picks are
// removed together when the election ends (gorge emits one batched event
// per kind then). The kinds are offered in kernel kind order; the oracle
// compares them by kind (gorge's option order is its carrier's slice
// order, a presentation detail). The election's picks ride res.peek
// (counter kinds, at most len(peek)) and res.left (the counters still to
// remove); 0 B.

const raRemCtrAny uint8 = 252

// ChRemCtrKind is gorge's "counter_kind" KChoose: mode = the counter kind
// (AkDone: gorge's remove_counter_done).
const ChRemCtrKind ChoiceKind = 252

// rcaStart begins the election; false: the ask is posed.
func (s *State) rcaStart(k *prep, in *kop.Instr) bool {
	r := &s.res
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return true
	}
	n := s.value(k, in.B)
	if n <= 0 {
		return true
	}
	r.left = uint8(min(n, len(r.peek)))
	r.peekN = 0
	return s.rcaStep(k, in, sl)
}

// rcaPending is the picks so far of kind c.
func (s *State) rcaPending(c int) int {
	n := 0
	for i := 0; i < int(s.res.peekN); i++ {
		if int(s.res.peek[i]) == c {
			n++
		}
	}
	return n
}

// rcaKinds lists the kinds of the permanent at sl still present after the
// picks so far, in kernel kind order.
func (s *State) rcaKinds(sl Slot, out *[kop.NumCounterKinds3]uint8) int {
	return s.ctrKindsOn(sl, out, true)
}

// ctrKindsOn lists the counter kinds of the permanent at sl in kernel kind
// order; pending subtracts the removal picks so far.
func (s *State) ctrKindsOn(sl Slot, out *[kop.NumCounterKinds3]uint8, pending bool) int {
	n := 0
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		if !kop.IsCounterKind(c) || c == int(kop.CtrOther) {
			continue
		}
		have := s.perms[sl].ctrN(s, c)
		if pending {
			have -= s.rcaPending(c)
		}
		if have > 0 {
			out[n] = uint8(c)
			n++
		}
	}
	return n
}

// rcaStep takes the next pick: unasked when one kind is left without
// UpTo$, asked otherwise; at the end it removes the picks. false: asked.
func (s *State) rcaStep(k *prep, in *kop.Instr, sl Slot) bool {
	r := &s.res
	var kinds [kop.NumCounterKinds3]uint8
	for r.left > 0 {
		n := s.rcaKinds(sl, &kinds)
		if n == 0 {
			break
		}
		if n > 1 || in.C&1 != 0 { // seat-ok: an instruction flag bit
			r.ask, r.askP = raRemCtrAny, r.e.ctrl
			return false
		}
		r.peek[r.peekN] = Obj(kinds[0])
		r.peekN++
		r.left--
	}
	s.rcaFinish(sl)
	return true
}

// rcaFinish removes the picked counters.
func (s *State) rcaFinish(sl Slot) {
	r := &s.res
	if r.peekN > 0 {
		s.touchSl(sl)
		p := &s.perms[sl]
		p.seeCtr(s) // gorge lists a kind removed to none at 0
		for i := 0; i < int(r.peekN); i++ {
			p.subCtrN(s, int(r.peek[i]), 1)
		}
	}
	r.peekN, r.left = 0, 0
}

// askOptionsRca builds the kind ask (the tail of askOptionsStdmech).
//
//go:noinline
func (s *State) askOptionsRca(k *prep, d *Decision) bool {
	if s.res.ask != raRemCtrAny {
		return s.askOptionsPiles(k, d) // act_piles.go
	}
	in := &s.prog(k)[s.res.pc]
	d.Sub = ChRemCtrKind
	put := in.Ext() == kop.ExtPutCounterExisting // ctr_putexisting.go
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		var kinds [kop.NumCounterKinds3]uint8
		n := s.ctrKindsOn(sl, &kinds, !put)
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, 0, 0, kinds[i]))
		}
	}
	if in.C&1 != 0 && !put { // seat-ok: an instruction flag bit
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// applyAskRca takes one pick (or Done) and steps the election on.
//
//go:noinline
func (s *State) applyAskRca(k *prep, a Action) bool {
	if s.res.ask != raRemCtrAny {
		return s.applyAskPiles(k, a) // act_piles.go
	}
	r := &s.res
	r.ask = raNone
	in := &s.prog(k)[r.pc]
	if in.Ext() == kop.ExtPutCounterExisting {
		s.pceAnswer(k, in, a) // ctr_putexisting.go
		return true
	}
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		r.peekN, r.left = 0, 0
		s.continueRes(k, 0)
		return true
	}
	if a.Kind() == AkDone {
		s.rcaFinish(sl)
		s.continueRes(k, 0)
		return true
	}
	r.peek[r.peekN] = Obj(a.Mode())
	r.peekN++
	r.left--
	if s.rcaStep(k, in, sl) {
		s.continueRes(k, 0)
	}
	return true
}
