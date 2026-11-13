package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext CounterLose (Tishana's Tidebinder): "counter up to one target
// activated or triggered ability. If an ability of an artifact, creature, or
// planeswalker is countered this way, that permanent loses all abilities for
// as long as CARDNAME remains on the battlefield". gorge's Counter with
// RememberCountered$ remembers the countered ability's source card; the
// following Effect (ConditionPresent over Remembered on the battlefield,
// Duration$ AsLongAsInPlay) is a host-bound LoseAbilities override on it
// (the WhileHost records). 0 B: the source is read off the
// stack entry before the counter removes it.

// counterLose is ExtCounterLose.
//
//go:noinline
func (s *State) counterLose(k *prep, in *kop.Instr) {
	slot, ok := tgtSlot(in.Obj)
	if !ok {
		return
	}
	c := s.tgtCode(slot)
	if !isStackTarget(c) {
		return
	}
	i := s.stackTgtIdx(c, s.res.e.tver[slot])
	if i < 0 {
		return
	}
	src, ver := s.stack[i].obj, s.stack[i].srcVer
	// a creator-sourced delayed trigger (an AtEOT$ rider, CR 603.7e) stamps
	// the affected object's ver, not its creator's: gorge remembers the
	// source by id, so its live incarnation is the one read
	if st := &s.stack[i]; st.ab == abDelayed && dlyRef(src, st.trigObj) != src && src != NoObj && int(src) < MaxObj {
		ver = s.objs[src].ver
	}
	s.g.stkCt = s.stack[i].ctrl + 1
	s.counterAt(k, i)
	if src == NoObj || int(src) >= MaxObj {
		return
	}
	ob := &s.objs[src]
	if ob.zone != kop.ZoneBattlefield || ob.ver != ver {
		return
	}
	if s.typesGen(k, ob.slot)&(kop.TArtifact|kop.TCreature|kop.TPlaneswalker) == 0 {
		return
	}
	e := &s.res.e
	if !s.hostHere(e) {
		return
	}
	s.applyOverridePT(k, ob.slot, int(in.C), e.card, 0, int16(e.obj), int16(e.srcVer))
}
