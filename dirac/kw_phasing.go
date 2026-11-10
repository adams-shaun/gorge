package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Phasing (CR 702.26, Spectral Adversary's "up to that many other
// target artifacts, creatures, and/or enchantments phase out"). gorge keeps a
// phased-out permanent in its battlefield list with Object.PhasedOut set and
// every reader skips it. The kernel gets the same exclusion without a test in
// any walk: a phased-out permanent keeps its slot and perm record, but it is
// taken off every slot mask (occ, ctrlM, the trigger and static subscriber
// masks) and its object's zone becomes kop.ZonePhased (phased out directly)
// or kop.ZonePhasedInd (phased out indirectly, CR 702.26g: it was attached to
// a permanent that phased out). Every walk that ranges over the masks, and
// every check of "is it on the battlefield" (a target's legality, a Self
// ref), then ignores it. Phasing is no zone change (CR 702.26d): the object
// keeps its version, so a stack object that targets it sees the same object,
// just not on the battlefield, and nothing "enters" or "leaves".
//
// The cold paths that must still see a phased-out permanent are guarded by
// prep.phasing, set only for a table with an ExtPhases program: the slot
// allocator (a hidden slot is not free), the untap step (the phase-in scan and
// the sickness clear of the turn's new active player), the cleanup step
// (damage and until-end-of-turn effects end on it too), a host's departure
// (its hidden attachments lose it) and the projection. All of it is cold and
// out of line.

// tablePhasing reports whether some program of t is ExtPhases.
func tablePhasing(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtPhases {
					return true
				}
			}
		}
	}
	return false
}

// hiddenAt reports whether slot sl holds a phased-out permanent.
func (s *State) hiddenAt(sl Slot) bool {
	ob := &s.objs[s.perms[sl].obj]
	return ob.zone.Phased() && ob.slot == sl
}

// phasedSlots is the set of slots holding phased-out permanents.
//
//go:noinline
func (s *State) phasedSlots() slotMask {
	var m slotMask
	for sl := 0; sl < MaxPerm; sl++ {
		if s.hiddenAt(Slot(sl)) {
			m.set(Slot(sl))
		}
	}
	return m
}

// freeSlots is the battlefield slots free for an entrant: not occupied and
// not holding a phased-out permanent.
//
//go:noinline
func (s *State) freeSlots() slotMask { return s.occ.or(s.phasedSlots()).not() }

// phasesExt is ExtPhases: the permanent Obj names phases out (gorge
// effPhases without the toggle: an already phased-out one is skipped, which a
// target an earlier object's attachment sweep took is too).
//
//go:noinline
func (s *State) phasesExt(k *prep, in *kop.Instr) {
	if DebugPhasesNone {
		return
	}
	if in.Obj == kop.RefNone {
		s.phasesAll(k, in.F)
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot && s.occ.has(sl) {
		s.phaseOut(k, sl, false)
	}
}

// DebugPhasesNone (test only, TestCityOnTheEdgeMutation): ExtPhases phases
// nothing out, so the permanents gorge phases out stay in play here and the
// lockstep diverges.
var DebugPhasesNone bool

// phasesAll is ExtPhases with Obj RefNone (AllValid$ <filter>; gorge
// effPhases' arena walk): every battlefield permanent f admits, in object
// (ObjID) order, phases out one at a time. The set is read before the first
// phases out, as gorge reads it, but a permanent an earlier host's
// attachment sweep already phased out indirectly is skipped (gorge's
// !PhasedOut guard), so an Equipment the sweep reaches before the creature
// it is attached to goes out directly and one reached after goes out with
// it.
//
//go:noinline
func (s *State) phasesAll(k *prep, f kop.FilterID) {
	var hit slotMask
	src := s.objRef(k, kop.RefSelf)
	ctrl := s.res.e.ctrl
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPermGen(k, f, sl, ctrl, src, layerFull) {
			hit = hit.with(sl)
		}
	}
	var ord [MaxPerm]Slot
	n := 0
	for m := hit; m.any(); m = m.rest() {
		sl := m.first()
		j := n
		for j > 0 && s.perms[ord[j-1]].obj > s.perms[sl].obj {
			ord[j] = ord[j-1]
			j--
		}
		ord[j] = sl
		n++
	}
	for i := 0; i < n; i++ {
		if sl := ord[i]; s.occ.has(sl) {
			s.phaseOut(k, sl, false)
		}
	}
}

// phaseInBlocked reports whether a StatCantPhaseIn on the battlefield forbids
// every phase-in (gorge phaseBlocked, rules/cantphase.go; the static's
// ValidCard$ is the blanket Card.phasedOutPermanent). A source that lost its
// abilities does not apply (eachStatic).
//
//go:noinline
func (s *State) phaseInBlocked(k *prep) bool {
	if k.gen == nil {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {

		if st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatCantPhaseIn {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// phaseOut phases the permanent at sl out, and every permanent attached to
// it indirectly (gorge foldPhaseOut, CR 702.26g). It leaves combat (CR
// 702.26b) the way a departure does.
//
//go:noinline
func (s *State) phaseOut(k *prep, sl Slot, indirect bool) {
	p := &s.perms[sl]
	o := p.obj
	s.touch(o)
	if p.flags&pfAttacking != 0 {
		s.nAttackers--
	}
	s.occ.clr(sl)
	if MaxSeats == 2 {
		s.ctrlM[0].clr(sl)
		s.ctrlM[1].clr(sl)
	} else {
		for q := range s.ctrlM {
			s.ctrlM[q].clr(sl)
		}
	}
	for m := s.occ; m.any(); m = m.rest() {
		q := m.first()
		qp := &s.perms[q]
		if qp.blocking == sl {
			qp.blocking = NoSlot
			s.touchSl(q)
		}
		if p.flags&pfBlocking != 0 && p.blocking == q && qp.nblock > 0 {
			qp.nblock--
			s.touchSl(q)
		}
	}
	p.flags &^= pfAttacking | pfBlocking | pfBlocked | pfDealtFirst
	p.blocking, p.nblock = NoSlot, 0
	// the card the permanent wears (a copy's statics and triggers too,
	// tables): phase-out takes the worn card off the masks (CR 702.26b)
	s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
	z := kop.ZonePhased
	if indirect {
		z = kop.ZonePhasedInd
	}
	s.objs[o].zone = z
	for m := s.occ; m.any(); m = m.rest() {
		if q := m.first(); s.occ.has(q) && s.perms[q].attached == sl {
			s.phaseOut(k, q, true)
		}
	}
}

// phaseIn phases the permanent at sl in, and the permanents that phased out
// indirectly with it (gorge foldPhaseOut Amount -1, CR 702.26h); they stay
// attached.
//
//go:noinline
func (s *State) phaseIn(k *prep, sl Slot) {
	p := &s.perms[sl]
	s.touch(p.obj)
	s.objs[p.obj].zone = kop.ZoneBattlefield
	s.occ.set(sl)
	s.ctrlM[SeatIx(p.ctrl)].set(sl)
	// whatever the permanent wears now: a copy that began or ended while
	// phased out is subscribed as it phases in (CR 702.26c)
	s.subscribe(sl, &k.cards[s.cardAt(sl)], true)
	for q := 0; q < MaxPerm; q++ {
		if qs := Slot(q); s.perms[qs].attached == sl && s.hiddenAt(qs) && s.objs[s.perms[qs].obj].zone == kop.ZonePhasedInd {
			s.phaseIn(k, qs)
		}
	}
}

// phaseUntap is the untap step's phasing action (gorge finishUntapStep, CR
// 702.26a/p): each phased-out permanent of the active player, in their
// battlefield list's order (entry order, perm.ts), phases in unless it phased
// out indirectly, unless a CantPhaseIn on the board as the step began forbids
// the set (read once: the phase-ins are simultaneous). The turn change also
// ended the summoning sickness of every permanent the new active player
// controls, phased out or not (gorge TurnChange); the untap loop that follows
// clears it on the phased-in ones.
//
//go:noinline
func (s *State) phaseUntap(k *prep) {
	var order [MaxPerm]Slot
	n := 0
	for sl := 0; sl < MaxPerm; sl++ {
		q := Slot(sl)
		if s.perms[q].ctrl != s.active || !s.hiddenAt(q) {
			continue
		}
		s.perms[q].flags &^= pfSick
		j := n
		for j > 0 && s.perms[order[j-1]].ts > s.perms[q].ts {
			order[j] = order[j-1]
			j--
		}
		order[j] = q
		n++
	}
	// The phase-ins are one simultaneous action (CR 502.1, 702.26a): the
	// CantPhaseIn block is read once, off the board before any of them, so a
	// phased-out source (a Human token of The City on the Edge of Forever)
	// does not hold back its siblings when it phases in earlier in the step.
	blocked := s.phaseInBlocked(k)
	for i := 0; i < n; i++ {
		if q := order[i]; !blocked && s.objs[s.perms[q].obj].zone == kop.ZonePhased {
			s.phaseIn(k, q)
		}
	}
	if n != 0 && !blocked && k.gen != nil {
		// The phase-ins put slots back on occ, so the override-derived
		// types and the layer walk the refresh recomputed while they were
		// phased out are stale until the next instruction would refresh
		// them; a decision asked before that (a trigger during the untap
		// step) would project the printed types (CR 702.26c: the game
		// treats a phased-in permanent as existing again).
		s.refreshTy(k)
	}
}

// phasedHostLeft unattaches the phased-out attachments of the permanent at
// sl, which is leaving; they are put in the graveyard (an Aura) or stay
// (Equipment) when they phase in, as a visible attachment is at the next
// state-based check.
//
//go:noinline
func (s *State) phasedHostLeft(sl Slot, host Obj) {
	for q := 0; q < MaxPerm; q++ {
		if qs := Slot(q); s.perms[qs].attached == sl && s.hiddenAt(qs) {
			s.perms[qs].attached = NoSlot
			s.touchSl(qs)
			if s.g.on {
				s.lingerPut(qs, host)
			}
		}
	}
}

// phasedCtrl is the phased-out permanents player p controls.
//
//go:noinline
func (s *State) phasedCtrl(p uint8) slotMask {
	var m slotMask
	for sl := 0; sl < MaxPerm; sl++ {
		if s.perms[sl].ctrl == p && s.hiddenAt(Slot(sl)) {
			m.set(Slot(sl))
		}
	}
	return m
}

// phasedPT is the P/T gorge projects for a phased-out creature: no
// continuous effect applies to it (CR 702.26b; gorge matchesWithCharsPT), so
// its base P/T plus its +1/+1 and -1/-1 counters. The projection alone reads
// it; nothing in play sees a phased-out permanent.
//
//go:noinline
func (s *State) phasedPT(k *prep, sl Slot) (int, int) {
	p := &s.perms[sl]
	c := p.ctrLow(int(kop.CtrP1P1)) - p.ctrLow(int(kop.CtrM1M1))
	if s.faceDown(k, sl) {
		return 2 + c, 2 + c // it stays face down (CR 110.5, 708.2)
	}
	bp, bt := s.phasedBasePT(k, sl)
	if s.fxM.has(sl) {
		if ep, et, ok := s.copyExcPT(k, sl); ok {
			bp, bt = ep, et // a copy exception's P/T is copiable (CR 707.9b)
		}
	}
	return bp + c, bt + c
}

// phasedBasePT is a phased-out permanent's printed P/T, or its own
// characteristic-defining P/T. A CDA is not one of the continuous effects
// CR 702.26b excludes: it defines the object's characteristics in every zone
// (CR 604.3), and gorge's CDASetPT reads it whatever the PhasedOut status, so
// the projection follows it. Every other override and layer is skipped.
//
//go:noinline
func (s *State) phasedBasePT(k *prep, sl Slot) (int, int) {
	p := &s.perms[sl]
	cid := s.cardAt(sl)
	pc := &k.cards[cid]
	bp, bt := int(pc.power), int(pc.tough)
	if k.gen == nil || !k.gen.has(cid, gfCDAPT) || s.genLost(k, sl) {
		return bp, bt
	}
	cd := &k.t.Cards[cid]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCDAPT {
			ctx := evalCtx{src: p.obj, srcSl: sl, ctrl: p.ctrl}
			bp, bt = s.evalV(k, a.Static.VA, ctx), s.evalV(k, a.Static.VB, ctx)
		}
	}
	return bp, bt
}
