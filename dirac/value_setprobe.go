package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ProbeSetExpr evaluates set expression e over the probe overlay ps (whose
// filters its SetAndFilter steps index) and lists its members in the order an
// op walks them: an object as its index, a player as PlayerCode. With top it
// reads in the context of the stack's top entry (its source, controller,
// targets and trigger referents), and reports false when the stack is empty;
// otherwise for source src (NoObj for none) controlled by ctrl, with no entry.
// The oracle's set atoms read it beside gorge's Defined$; the kernel never
// calls it.
func (s *State) ProbeSetExpr(ps *ProbeSet, e kop.SetExpr, src Obj, ctrl uint8, top bool) ([]TCode, bool) {
	ev := evalCtx{src: src, srcSl: s.probeSlot(src), ctrl: ctrl}
	if top {
		if s.sp == 0 {
			return nil, false
		}
		ev = s.ctxEntry(&s.stack[s.sp-1])
	}
	v := s.evalSetExpr(ps.k, e, setCtx{ev: ev})
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	n := s.setMembers(&v, ev.ctrl, &list, &vers)
	return append([]TCode(nil), list[:n]...), true
}

// ProbeStackTop is the source object and controller of the stack's top entry,
// false when the stack is empty.
func (s *State) ProbeStackTop() (Obj, uint8, bool) {
	if s.sp == 0 {
		return NoObj, 0, false
	}
	e := &s.stack[s.sp-1]
	return e.obj, e.ctrl, true
}
