package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ext PutCounterExisting (gorge putCounterExisting, CounterType$
// ExistingCounter): "choose a counter on target enchantment you control, put
// another of that kind on it" (Dathon, His Heart Open). Obj is the
// recipient and B the amount. The kinds on the recipient are listed in
// kernel kind order (the same walk as the Glissa kind ask, without its
// pending picks): none does nothing, one is put unasked, two or more ask the
// resolving controller (raRemCtrAny / ChRemCtrKind, branching on the
// instruction at res.pc). The pick rides the Action's mode byte; res.peek[0]
// holds it only across the CR 616.1 order ask, whose answer runs this
// instruction again. The put is an ordinary OpPutCounter of the picked kind:
// replacements, the order ask, ctrNote and the counter-added triggers.

// PutExistingStats counts the lockstep test's coverage: kind asks posed.
var PutExistingStats struct{ Asked atomic.Int64 }

// pceStart runs the instruction; false when the kind ask or the CR 616.1
// order ask is posed.
func (s *State) pceStart(k *prep, in *kop.Instr) bool {
	r := &s.res
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return true
	}
	if s.g.ro.fin && s.g.ro.ac&ctrOrdTag == ctrOrdTag {
		return s.pcePut(k, in, r.peek[0]) // the order answered: the same pick again
	}
	var kinds [kop.NumCounterKinds3]uint8
	switch s.ctrKindsOn(sl, &kinds, false) {
	case 0:
		return true
	case 1:
		return s.pcePut(k, in, Obj(kinds[0])) // code-ok: a counter kind
	}
	r.ask, r.askP = raRemCtrAny, r.e.ctrl
	PutExistingStats.Asked.Add(1)
	return false
}

// pceAnswer takes the kind pick and puts the counter.
func (s *State) pceAnswer(k *prep, in *kop.Instr, a Action) {
	s.res.ask = raNone
	if s.pcePut(k, in, Obj(a.Mode())) { // code-ok: a counter kind
		s.continueRes(k, 0)
	} else {
		s.ctl = ctlResolve // the CR 616.1 order ask
	}
}

// pcePut puts the amount of counters of the picked kind; false when the put
// parked on the order ask.
func (s *State) pcePut(k *prep, in *kop.Instr, kind Obj) bool {
	s.res.peek[0] = kind
	s.g.lastCK = kop.CounterKind(kind) // code-ok: a counter kind (CounterAddedOnce reads it)
	pi := kop.Instr{Op: kop.OpPutCounter, Obj: in.Obj, A: in.B, C: int16(kind)}
	return s.execPutCounter(k, &pi)
}
