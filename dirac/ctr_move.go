package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// moveCounter is OpMoveCounter (gorge effMoveCounter with one origin and
// one destination): up to A counters of kind B.N (A < 0: all of them) leave
// the origin in Ref C and the destination Obj gets that many, through the
// counter replacements (a CounterChange emit in gorge), firing its
// counter-added triggers.
func (s *State) moveCounter(k *prep, in *kop.Instr) {
	src := s.objRef(k, kop.Ref(in.C))
	dst := s.objRef(k, in.Obj)
	if src == NoSlot || dst == NoSlot || src == dst {
		return
	}
	kind := int(in.B.N)
	ps := &s.perms[src]
	have := ps.ctrN(s, kind)
	n := have
	if in.A.N >= 0 || in.A.K != kop.VConst {
		n = s.value(k, in.A)
	}
	if n > have {
		n = have
	}
	if n <= 0 {
		return
	}
	ps.seeCtr(s)
	ps.subCtrN(s, kind, n)
	s.touchSl(src)
	n = s.placeCounters(k, dst, kind, n, s.perms[dst].ctrl, ctrPutEffect) // as OpPutCounter

	pd := &s.perms[dst]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, dst, kind)
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.g.lastCK = kop.CounterKind(kind)
		s.fireObj(k, kop.TrigCounterAddedOnce, dst, pd.obj, pd.ctrl, int16(n))
	}
}
