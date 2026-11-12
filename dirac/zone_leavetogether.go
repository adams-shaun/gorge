package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A ChangeZone over several targets (Calamitous Tide's "return up to two target
// creatures to their owners' hands") is one simultaneous departure (CR 603.10a):
// gorge's BatchDepartures shares one board snapshot among the moves, so a
// permanent that leaves together with another sees it leave. Dour Port-Mage
// bounced beside Plumecreed Mentor still draws for it. Run one target at a
// time, the first to go would already be off the battlefield when the second's
// leaves triggers are checked.

// Destroy over several targets is the same departure (Wear Down destroying a
// Food-ified creature with the Sugar Coat that makes it one): destroySpared
// settles which survive, the rest leave together.

// leaveTogether moves every battlefield object of target slot i that a plain
// bounce, exile or destroy instruction names: the leaves triggers of all of them are
// checked first, in pick order, and only then do they leave. It reports false
// (nothing done) unless two or more objects are there to go and every one of
// them is on the battlefield; the per-target loop then runs as before.
func (s *State) leaveTogether(k *prep, in *kop.Instr, i uint8) bool {
	var z kop.Zone
	destroy := false
	switch {
	case in.Op == kop.OpBounce && in.C == 0:
		z = kop.ZoneHand
	case in.Op == kop.OpExile && in.C == 0:
		z = kop.ZoneExile
	case in.Op == kop.OpDestroy && in.C&2 == 0:
		z, destroy = kop.ZoneGraveyard, true
	default:
		return false
	}
	e := &s.res.e
	saveT, saveV, saveBad := e.tgt[i], e.tver[i], s.res.bad
	var objs [maxXTgt + 1]Obj
	n := 0
	ok := true
	add := func() {
		o := s.objRefObj(in.Obj)
		if o == NoObj || s.freeQueued(o) || s.freeBegun(o) {
			return
		}
		if s.objs[o].zone != kop.ZoneBattlefield {
			ok = false
			return
		}
		objs[n] = o
		n++
	}
	add()
	for j := 0; j < int(s.xtOf(e).n) && ok; j++ {
		if s.xtOf(e).slot[j] != i || !s.xtLegalRes(k, e, j, i) {
			continue
		}
		e.tgt[i], e.tver[i] = s.xtOf(e).code[j], s.xtOf(e).ver[j]
		s.res.bad &^= 1 << i
		add()
	}
	e.tgt[i], e.tver[i], s.res.bad = saveT, saveV, saveBad
	if !ok || n < 2 {
		return false
	}
	var zs [maxXTgt + 1]kop.Zone
	if destroy {
		if k.gen != nil {
			for _, o := range objs[:n] {
				if s.graveMoveCompetes(k, o) || s.parkedWaitingOn(o) {
					return false // the per-target path parks the move for its ask
				}
			}
		}
		m := 0
		for _, o := range objs[:n] {
			if !s.destroySpared(k, s.objs[o].slot, in.C&1 != 0) {
				objs[m] = o
				m++
			}
		}
		n = m
		for j, o := range objs[:n] {
			zs[j] = s.dieZone(k, s.objs[o].slot)
		}
	} else {
		for j := range zs[:n] {
			zs[j] = z
		}
	}
	for j, o := range objs[:n] {
		if s.res.active && k.gen != nil {
			s.snapResTarget(o)
		}
		s.fireLeave(k, s.objs[o].slot, zs[j])
	}
	for j, o := range objs[:n] {
		s.removePerm(k, s.objs[o].slot, zs[j], false)
	}
	return true
}
