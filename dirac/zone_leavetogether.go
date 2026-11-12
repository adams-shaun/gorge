package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A ChangeZone or Destroy over several targets (Calamitous Tide's "return up to
// two target creatures to their owners' hands", Wear Down's two artifacts and/or
// enchantments) is one simultaneous departure (CR 603.10a):
// gorge's BatchDepartures shares one board snapshot among the moves, so a
// permanent that leaves together with another sees it leave. Dour Port-Mage
// bounced beside Plumecreed Mentor still draws for it. Run one target at a
// time, the first to go would already be off the battlefield when the second's
// leaves triggers are checked.

// leaveTogether moves every battlefield object of target slot i that a plain
// bounce, exile or destroy instruction names: the leaves triggers of all of them
// are checked first, in pick order, and only then do they leave. A destroy
// settles each target's indestructible, regeneration and shield first, in pick
// order (gorge effDestroy builds its victim list before the first move); only
// the ones that die leave. It reports false
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
	case in.Op == kop.OpDestroy && in.C == 0:
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
	if destroy {
		s.destroyTogether(k, objs[:n])
		return true
	}
	for _, o := range objs[:n] {
		if s.res.active && k.gen != nil {
			s.snapResTarget(o)
		}
		s.fireLeave(k, s.objs[o].slot, z)
	}
	for _, o := range objs[:n] {
		s.removePerm(k, s.objs[o].slot, z, false)
	}
	return true
}

// destroyTogether is leaveTogether's departure for a destroy: the targets that
// are not absorbed go to the graveyard (or wherever a dies replacement sends
// them) as one batch. A move that must wait for an ordering ask (a parked
// exile-on-leave choice) is not modelled as a batch: the targets are then
// destroyed one at a time, as before.
func (s *State) destroyTogether(k *prep, objs []Obj) {
	if len(s.cold.parked) != 0 {
		s.destroyEach(k, objs)
		return
	}
	for _, o := range objs {
		if s.graveMoveCompetes(k, o) {
			s.destroyEach(k, objs)
			return
		}
	}
	var die [maxXTgt + 1]Obj
	var dz [maxXTgt + 1]kop.Zone
	n := 0
	for _, o := range objs {
		sl := s.objs[o].slot
		if s.destroyAbsorbed(k, sl) {
			continue
		}
		die[n], dz[n] = o, s.dieZone(k, sl)
		n++
	}
	for _, o := range die[:n] {
		if s.res.active && k.gen != nil {
			s.snapResTarget(o)
		}
	}
	for i, o := range die[:n] {
		s.fireLeave(k, s.objs[o].slot, dz[i])
	}
	for i, o := range die[:n] {
		s.removePerm(k, s.objs[o].slot, dz[i], false)
	}
}

// destroyEach destroys objs one at a time (destroy).
func (s *State) destroyEach(k *prep, objs []Obj) {
	for _, o := range objs {
		if s.objs[o].zone == kop.ZoneBattlefield {
			s.destroy(k, s.objs[o].slot)
		}
	}
}
