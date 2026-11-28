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

// ProbeRefBinding is a generated resolving context for the SetRefBase atoms:
// the source and controller of a triggered ability of the source, and the
// referents its Refs read. Src and TrigObj name objects (NoObj for no trigger
// object), TrigPl and DefSeat seats (the trigger event's player and the
// defending seat), Chosen the chosen card (NoObj for none) and ChosenPl the
// chosen player plus one (0 for none).
type ProbeRefBinding struct {
	Src, TrigObj, Chosen Obj
	Ctrl, TrigPl         uint8
	DefSeat, ChosenPl    uint8
}

// ProbeSetResolving evaluates e over the probe overlay ps as b's ability
// resolving -- SetRefBase reads the resolving entry, as an op's subject read
// does -- lists the members in walk order, and restores the state. Src must be
// a live object. The oracle's referent atoms read it; the kernel never calls
// it.
func (s *State) ProbeSetResolving(ps *ProbeSet, e kop.SetExpr, b ProbeRefBinding) []TCode {
	res, chosen, nchosen := s.res, s.g.chosen[0], s.g.nchosen
	defer func() { s.res, s.g.chosen[0], s.g.nchosen = res, chosen, nchosen }()
	ent := stackEntry{kind: seTrigger, obj: b.Src, srcVer: s.objs[b.Src].ver, card: s.objs[b.Src].card,
		ctrl: b.Ctrl, trigObj: b.TrigObj, trigPl: b.TrigPl, defSeat: b.DefSeat + 1}
	for i := range ent.tgt {
		ent.tgt[i] = NoTarget
	}
	s.res = resCtx{e: ent, active: true, lastTok: NoObj, eachObj: NoObj, chosen: objWord(b.Chosen), chPl: b.ChosenPl}
	s.g.chosen[0], s.g.nchosen = b.Chosen, 0
	if b.Chosen != NoObj {
		s.g.nchosen = 1
	}
	v := s.evalSetExpr(ps.k, e, setCtx{ev: s.ctxEntry(&s.res.e)})
	var list [MaxObj + MaxSeats]TCode
	var vers [MaxObj + MaxSeats]uint8
	n := s.setMembers(&v, ent.ctrl, &list, &vers)
	return append([]TCode(nil), list[:n]...)
}
