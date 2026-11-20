package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ops that read RefSet (kop.RefSet; set_eval.go names the members,
// set_iter.go walks them). execGen sends an instruction whose Obj is RefSet to
// execSet; each arm runs the op's single-object body once per member, the
// same ...On method the single-object path calls, so a set of one is the
// single-object op.
//
// Verdicts, per op (suspension-capable = setSuspOp, a member body can pose a
// decision, so the table holds coldSetIter):
//
//	OpPutCounter     members, suspension-capable: the CR 616.1 order ask (the instruction runs again)
//	OpPumpUntilEOT   members, one-shot, no decision
//	OpDestroy        members, suspension-capable: the leave parks a replacement-order ask
//	OpTap, OpUntap   members, no decision
//	OpExile, OpBounce (C 0), OpToLibrary (C top/bottom)
//	                 members, suspension-capable: the move parks a replacement-order ask
//	OpGainControl    members as one grant list (opGainControl's tail), no decision
//	OpGainLife, OpLoseLife
//	                 players, no decision
//	OpDraw           players: the seat chain a dredge draw already walks (dredgeDrawsOver)
//	OpDiscard        players: the seat chain of an each-player discard (discardSeq), an
//	                 unfiltered self-chosen discard only; the others are StallSetShape
//	OpSacrifice      players: the seat chain of an each-player sacrifice (startSac /
//	                 startOptSac), which asks only when a player has more than the amount
//
// A destroy or a move over a set runs member by member inside one batch
// (batchOpen, holdOpen) as execEachTarget's target loop does; gorge's
// simultaneous leave of a multi-object group (leaveTogether) is not modelled
// here.
func (s *State) execSet(k *prep, in *kop.Instr) bool {
	e, pc := &s.res.e, int(s.res.pc)
	switch in.Op {
	case kop.OpDraw, kop.OpGainLife, kop.OpLoseLife, kop.OpDiscard, kop.OpSacrifice:
		return s.execSetPlayers(k, in, e, pc)
	case kop.OpGainControl:
		return s.execSetControl(k, in, e, pc)
	}
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	switch in.Op {
	case kop.OpPutCounter:
		s.g.lastCK = kop.CounterKind(in.C) // CounterAddedOnce reads the kind
		old := s.ctrAllBatchOpen()         // the set is one action (trig_counteraddedall.go)
		ok := s.forSet(k, e, in, pc, (*State).putCounterMember)
		s.ctrAllBatchClose(old)
		return ok
	case kop.OpPumpUntilEOT:
		return s.forSet(k, e, in, pc, (*State).pumpMember)
	case kop.OpTap:
		defer s.tapBatchClose(s.tapBatchOpen(k)) // one Tap resolution is one tapping action (trig_tapall.go)
		return s.forSet(k, e, in, pc, (*State).tapMember)
	case kop.OpUntap:
		return s.forSet(k, e, in, pc, (*State).untapMember)
	case kop.OpDestroy:
		hr := s.holdOpen(k)
		defer s.holdClose(k, hr)
		return s.forSet(k, e, in, pc, (*State).destroyMember)
	case kop.OpExile, kop.OpBounce, kop.OpToLibrary:
		if in.Op == kop.OpBounce && in.C != 0 || in.Op == kop.OpToLibrary && in.C != 0 && in.C != 1 {
			s.stall(StallSetShape)
			return true
		}
		hr := s.holdOpen(k)
		defer s.holdClose(k, hr)
		return s.forSet(k, e, in, pc, (*State).moveMember)
	}
	panic("kernel: RefSet is not served by op " + in.Op.String())
}

// pumpOn is OpPumpUntilEOT's body for the creature at sl.
func (s *State) pumpOn(k *prep, in *kop.Instr, sl Slot) {
	s.touchSl(sl)
	s.eotAdd(k, sl, s.value(k, in.A), s.value(k, in.B))
}

// tapOn is OpTap's body for the permanent at sl.
func (s *State) tapOn(k *prep, in *kop.Instr, sl Slot) {
	was := s.perms[sl].flags & pfTapped
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	if was == 0 {
		if k.gen != nil && s.subs[kop.TrigTaps].any() {
			who := s.effTapper(sl)
			if in.W != 0 {
				// the instruction's Tapper$ provenance (gorge
				// effTapOrUntap's Tapper$ read; the election ask
				// is the controller's, the tap's is the ref's)
				if p := s.playerRef(k, in.W); p < s.nSeats() {
					who = p
				}
			}
			s.fireTapsBy(k, sl, who)
		}
		s.markTapped(k, sl) // only a tap that happens marks
	}
}

// untapOn is OpUntap's body for the permanent at sl.
func (s *State) untapOn(k *prep, sl Slot) {
	if s.untapBlocked(k, sl) {
		return
	}
	if p := &s.perms[sl]; p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
		p.seeCtr(s)
		s.subCtrFire(k, sl, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d)
		s.touchSl(sl)
		return
	}
	s.untapEffect(k, sl)
	s.touchSl(sl)
}

// bfSlot is the battlefield slot of member o, NoSlot when it is not a
// permanent (a member body acts on the permanent it finds, as objRef does).
func (s *State) bfSlot(o Obj) Slot {
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		return ob.slot
	}
	return NoSlot
}

// The member bodies: a RefSet member o is read as the permanent it is on the
// battlefield. They are method expressions passed to forSet.

func (s *State) putCounterMember(k *prep, in *kop.Instr, o Obj) bool {
	if sl := s.bfSlot(o); sl != NoSlot {
		return s.putCounterOnSlot(k, in, sl)
	}
	return true
}

func (s *State) pumpMember(k *prep, in *kop.Instr, o Obj) bool {
	if sl := s.bfSlot(o); sl != NoSlot {
		s.pumpOn(k, in, sl)
	}
	return true
}

func (s *State) tapMember(k *prep, in *kop.Instr, o Obj) bool {
	if sl := s.bfSlot(o); sl != NoSlot {
		s.tapOn(k, in, sl)
	}
	return true
}

func (s *State) untapMember(k *prep, in *kop.Instr, o Obj) bool {
	if sl := s.bfSlot(o); sl != NoSlot {
		s.untapOn(k, sl)
	}
	return true
}

func (s *State) destroyMember(k *prep, in *kop.Instr, o Obj) bool {
	if sl := s.bfSlot(o); sl != NoSlot {
		s.destroyOn(k, in, sl)
	}
	return true
}

// moveMember moves o to the op's fixed destination, from wherever it is.
func (s *State) moveMember(k *prep, in *kop.Instr, o Obj) bool {
	switch in.Op {
	case kop.OpExile:
		s.moveObjOn(k, kop.RefSet, o, kop.ZoneExile, false)
	case kop.OpBounce:
		s.moveObjOn(k, kop.RefSet, o, kop.ZoneHand, false)
	default:
		s.moveObjOn(k, kop.RefSet, o, kop.ZoneLibrary, in.C == 1)
	}
	return true
}

// execSetControl is OpGainControl over a set: every battlefield member, in
// set order, one grant list (opGainControl's tail).
func (s *State) execSetControl(k *prep, in *kop.Instr, e *stackEntry, pc int) bool {
	var v setVal
	if !s.setLoad(k, e, pc, &v, false) {
		return true
	}
	var list [MaxPerm]Slot
	n := 0
	for _, o := range v.objs[:v.n] {
		if sl := s.bfSlot(o); sl != NoSlot && n < len(list) {
			list[n] = sl
			n++
		}
	}
	if n != 0 {
		s.gainControlList(k, in, list[:n])
	}
	return true
}

// execSetPlayers is the ops whose set is a players set.
func (s *State) execSetPlayers(k *prep, in *kop.Instr, e *stackEntry, pc int) bool {
	switch in.Op {
	case kop.OpGainLife:
		return s.forSetPlayers(k, e, in, pc, (*State).gainLifeMember)
	case kop.OpLoseLife:
		s.g.lostAmt = 0
		ok := s.forSetPlayers(k, e, in, pc, (*State).loseLifeMember)
		return ok
	}
	var v setVal
	if !s.setLoad(k, e, pc, &v, true) {
		return true
	}
	var q seatSeq
	for _, o := range v.objs[:v.n] {
		q.add(uint8(o))
	}
	switch in.Op {
	case kop.OpDraw:
		if dredgeOn(k) {
			return s.dredgeDrawsOver(k, in, q)
		}
		n := s.value(k, in.A)
		for i := uint8(0); i < q.n; i++ {
			for j := 0; j < n && !s.over; j++ {
				s.dredgeDrawOne(in, q.s[i])
			}
		}
		return true
	case kop.OpDiscard:
		if in.F != 0 || in.C != kop.ChooseSelf {
			s.stall(StallSetShape)
			return true
		}
		if q.n == 0 {
			return true
		}
		return s.discardSeq(k, in, q)
	}
	// kop.OpSacrifice
	if q.n == 0 {
		return true
	}
	s.repRest(q, 1)
	if in.C&kop.OptAsk != 0 { // Optional$ (act_optionalask.go)
		return s.startOptSac(k, in, q.at(0))
	}
	return s.startSac(k, in, q.at(0))
}

func (s *State) gainLifeMember(k *prep, in *kop.Instr, p uint8) bool {
	s.gainLife(k, p, s.value(k, in.A))
	return true
}

// loseLifeMember adds the life lost to ExLastLifeLost's tally (the single-object
// op sets it once, after its players).
func (s *State) loseLifeMember(k *prep, in *kop.Instr, p uint8) bool {
	n := s.value(k, in.A)
	s.loseLife(k, p, n)
	s.g.lostAmt += int16(max(n, 0))
	return true
}
