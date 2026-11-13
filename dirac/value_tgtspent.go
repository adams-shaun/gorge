package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// snapTgtSpent records the cast spend of the spell the resolving spell
// targets, +1 (0 = no snapshot, saturating at 255), as the resolution
// starts. A Counter later in the chain removes the target's stack entry and
// with it the recorded spend, so a later condition must read this snapshot
// (gorge captureStackTargetSnapshots and targetManaSpentLKI; EOE Unravel).
//
//go:noinline
func (s *State) snapTgtSpent(e *stackEntry) {
	s.g.tgtSpent = 0
	if e.kind != seSpell || e.nt == 0 {
		return
	}
	t := e.tgt[0]
	if !isObjTarget(TCode(t)) || s.objs[t].zone != kop.ZoneStack {
		return
	}
	spent := int(s.castSpent(Obj(t)))
	if spent > 254 {
		spent = 254
	}
	s.g.tgtSpent = uint8(spent) + 1
}
