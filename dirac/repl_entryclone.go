package dirac

// The CR 616.1 order between an as-enters copy and another replacement over
// the same entry (Waxen Shapethief copying a permanent while Thunderous
// Velocipede gives the entrant a +1/+1 counter). gorge asks the order with
// the permanent spell still on the stack (engine/rules/replacement_choice.go,
// replChoiceEntryOrder; the affected player is the entrant's controller), so
// the kernel poses the same KReplacement ask before the move and applies the
// chosen source first.
//
// Only the two-candidate shape is modelled (the entrant's OpBecomeCopy and one
// other source): a longer competition keeps the old own-line-first order,
// which is what the kernel did before this ask existed.

// cloneEntryCands lists the replacements competing over the entry of the
// resolving permanent spell o, in gorge's scan order: the other permanents'
// matching enter replacements, then the entrant's own line (which follows its
// owner's battlefield when it enters from the stack). Only entryComp's
// other-source scan is reused: it does not add an entrant whose own body is
// neither a tap nor an untap, so the clone is appended here.
func (s *State) cloneEntryCands(k *prep, o Obj, ctrl uint8) (cand [8]Obj, n int) {
	c, _ := s.entryComp(k, o, ctrl, NoSlot, false)
	if c.over || int(c.n) >= len(cand) {
		return cand, 0
	}
	n = int(c.n)
	copy(cand[:n], c.cand[:n])
	for i := 0; i < n; i++ {
		if cand[i] == o {
			return cand, n
		}
	}
	cand[n] = o
	return cand, n + 1
}

// cloneEntryOrderPark poses the order ask for the permanent spell o, which
// ctrl is about to put onto the battlefield as a copy, when exactly one other
// source also matches its entry. It reports whether the resolution
// suspended on the ask.
func (s *State) cloneEntryOrderPark(k *prep, o Obj, ctrl uint8) bool {
	if s.g.pre.state != 2 || s.g.pre.kind != akClone || s.g.pre.obj != o || s.g.pre.ordSet {
		return false // no copy election was asked, or the order is already answered
	}
	if k.gen == nil || s.cloneOp(k, s.objs[o].card) == nil {
		return false
	}
	if _, n := s.cloneEntryCands(k, o, ctrl); n != 2 {
		return false
	}
	s.g.pre.kind, s.g.pre.state = akEntryOrder, 1
	s.g.pre.land = false
	s.g.pre.ordSet = true
	s.res.active = false
	s.g.ctlRet = ctlPriority
	s.ctl = ctlGenAsk
	return true
}

// cloneEntryOrderOptions fills the ask: one option per competing source, in
// gorge's scan order (the source the option applies first), re-derived from
// the state that asking does not change.
func (s *State) cloneEntryOrderOptions(k *prep, d *Decision) {
	cand, n := s.cloneEntryCands(k, s.g.pre.obj, uint8(d.Player))
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, uint8(cand[i]), 0, uint8(i))) // mode: the scan index
	}
}
