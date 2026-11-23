package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// AddOrRemoveCounterStats counts the lockstep test's coverage: elections
// posed, how many were the each-existing walk, and the arm answered (0
// remove, 1 put).
var AddOrRemoveCounterStats struct {
	Asked, EachExisting, Removed, Put atomic.Int64
}

// ExtAddOrRemoveCounter (gorge effAddOrRemoveCounter, effects/counters.go):
// "choose a counter on target permanent: remove it or put another on it".
// Obj is the target or Defined$ object, B the CounterNum$ amount, and C the
// named counter kind, or -1 for EachExistingCounter$ True (walk the kinds the
// object holds).
//
// C < 0 is the each-kind loop: the kinds the object holds are walked in the
// kernel's ascending kind order (heldKinds; gorge walks its counter slice's
// insertion order), one add/remove election per kind, in that order. An
// object holding two or more kinds stalls (StallCtrOrder): the first election
// is asked about a different kind on each side and the kernel keeps no
// insertion order to follow. Each
// election is gorge's two options -- remove first, then put -- so a no-host
// bot's first-option answer is gorge's no-host fallback (remove). An answer
// to the put arm goes through an ordinary OpPutCounter, so the AddCounter
// replacements, the CR 616.1 order ask and the counter-added triggers run,
// exactly like the other adding sites. The remove arm marks the kind seen and
// subtracts the amount clamped to what the object holds (gorge's
// aorApplyAct). No counter-removal trigger event exists yet (the
// CounterRemoved ticket), so the removal fires none, like the other removal
// sites.

// ChGenAddOrRemove is gorge's "aor_elect" KChoose: source = the object whose
// counter is chosen, mode 0 = remove / 1 = put. Explicit ordinal clear of the
// generated enum's tail.
const ChGenAddOrRemove ChoiceKind = 255

// raAddOrRemoveCounter is the resolution ask kind of the election (generated
// from dirac/manifest/ask/raAddOrRemoveCounter).
//
// The election's two options and the current kind ride res.peekI / res.left:
// peekI is the kind, left the answered arm (0 remove, 1 put). The each-kind
// walk keeps the last acted kind in res.chosen (255 = none yet) and
// res.peekN (1 = each-existing, 0 = named). None of these grows State.

// addOrRemoveCounter is ExtAddOrRemoveCounter.
//
//kmanifest:ext ExtAddOrRemoveCounter
func (s *State) addOrRemoveCounter(k *prep, in *kop.Instr) (ok, done bool) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return true, true
	}
	r := &s.res
	if s.g.ro.fin && s.g.ro.ac&ctrOrdTag == ctrOrdTag {
		// the CR 616.1 order ask answered: apply the same kind's arm again
		if !s.aorApply(k, in, sl) {
			s.ctl = ctlResolve
			return false, true
		}
		return s.aorNext(k, sl, false)
	}
	if in.C < 0 {
		if s.permRealCtrs(sl) > 1 {
			s.capFull(StallCtrOrder)
			return true, true
		}
		r.peekN, r.chosen = 1, 255
		return s.aorNext(k, sl, false)
	}
	r.peekN, r.peekI, r.chosen, r.left = 0, uint8(in.C), objWord(uint8(in.C)), 0
	r.ask, r.askP = raAddOrRemoveCounter, r.e.ctrl
	return false, true
}

// aorNext poses the next each-kind election, or ends the walk.
func (s *State) aorNext(k *prep, sl Slot, fromAnswer bool) (ok, done bool) {
	r := &s.res
	if r.peekN != 0 {
		if c, found := s.aorNextKind(sl); found {
			r.peekI, r.chosen, r.left = c, objWord(c), 0
			r.ask, r.askP = raAddOrRemoveCounter, r.e.ctrl
			return false, true
		}
	}
	if fromAnswer {
		s.continueRes(k, 0)
	}
	return true, true
}

// aorNextKind is the first held kind after res.chosen (255: the first kind).
func (s *State) aorNextKind(sl Slot) (uint8, bool) {
	var held ctrHeld
	p := &s.perms[sl]
	for _, c := range held[:p.heldKinds(s, &held)] {
		// heldKinds can carry the Class-level marker (no counter) and
		// CtrOther is a filler: gorge's counterKinds skips both, and the
		// other kernel walks filter the same way.
		if !kop.IsCounterKind(int(c)) || c == uint16(kop.CtrOther) {
			continue
		}
		if s.res.chosen == 255 || objWord(uint8(c)) > s.res.chosen {
			return uint8(c), true
		}
	}
	return 0, false
}

// aorOptions fills one election: remove first, then put (gorge's option
// order; the arm is the action's mode, the canonical form carries the kind).
//
//kmanifest:askopts raAddOrRemoveCounter
func (s *State) aorOptions(k *prep, d *Decision) {
	in := &s.prog(k)[s.res.pc]
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	o := s.perms[sl].obj
	d.Sub = ChGenAddOrRemove
	d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	d.add(MakeAction(AkChoose, TCode(o), 0, 1))
	AddOrRemoveCounterStats.Asked.Add(1)
	if s.res.peekN != 0 {
		AddOrRemoveCounterStats.EachExisting.Add(1)
	}
}

// aorAnswer applies one election and moves on.
//
//kmanifest:askapply raAddOrRemoveCounter
func (s *State) aorAnswer(k *prep, a Action) {
	r := &s.res
	r.ask, r.left = raNone, 0
	if a.Mode() == 1 {
		r.left = 1
		AddOrRemoveCounterStats.Put.Add(1)
	} else {
		AddOrRemoveCounterStats.Removed.Add(1)
	}
	in := &s.prog(k)[r.pc]
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		s.continueRes(k, 0)
		return
	}
	if !s.aorApply(k, in, sl) {
		s.ctl = ctlResolve // the CR 616.1 order ask
		return
	}
	s.aorNext(k, sl, true)
}

// aorApply runs the answered arm for the current kind. false: the put parked
// on the CR 616.1 order ask.
func (s *State) aorApply(k *prep, in *kop.Instr, sl Slot) bool {
	kind := int(s.res.peekI)
	if s.res.left == 1 {
		s.g.lastCK = kop.CounterKind(kind) // CounterAddedOnce reads the kind
		pi := kop.Instr{Op: kop.OpPutCounter, Obj: in.Obj, A: in.B, C: int16(kind)}
		return s.execPutCounter(k, &pi)
	}
	p := &s.perms[sl]
	n := s.value(k, in.B)
	if n < 0 {
		n = 0
	}
	if have := p.ctrN(s, kind); n > have {
		n = have
	}
	if n <= 0 {
		return true
	}
	s.touchSl(sl)
	p.seeCtr(s)                  // gorge lists a kind removed to none at 0
	s.subCtrFire(k, sl, kind, n) // trig_counterremoved.go
	return true
}

// FeatAddOrRemoveKind is the counter kind of the pending AddOrRemoveCounter
// election, for the oracle's canonical comparison.
func (s *State) FeatAddOrRemoveKind() uint8 { return s.res.peekI }
