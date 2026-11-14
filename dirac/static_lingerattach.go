package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// maxLinger is the live lingering-attachment records a state keeps. An Aura
// or Equipment whose host leaves the battlefield keeps showing that host in
// a projection until the next state-based pass, which drains the whole set
// (genState.linger). Each leaving host records only the permanents attached
// to it, so the live set is small; eight records cover a board wipe with
// several equipped creatures. Past eight the insert stalls (StallLinger)
// instead of dropping the attachment.
const maxLinger = 8

// lingerRec is one live lingering attachment: the attached permanent's slot
// and the object of the host that left. lingerM says which slots have a
// live record; the record's slot bytes are stale once the mask drops it.
type lingerRec struct {
	sl  Slot
	obj Obj
}

// lingerPut records that slot q's Aura/Equipment lost host o. It overwrites
// q's existing record (the attachment lost a second host in the same
// window) and otherwise reuses a record whose slot lingerM no longer names:
// the mask is the liveness oracle, so a record is free even while its stale
// slot bytes still name an old slot. nlinger bounds the records ever
// written; the SBA drain resets it with the mask. Cold: only a leaving host
// reaches it.
//
//go:noinline
func (s *State) lingerPut(q Slot, o Obj) {
	s.noteLastBear(q, o) // the departed attachment's CR 608.2h bearer
	g := &s.g
	free := -1
	for i := uint8(0); i < g.nlinger; i++ {
		r := &g.linger[i]
		if r.sl == q {
			r.obj = o
			g.lingerM.set(q)
			return
		}
		if free < 0 && !g.lingerM.has(r.sl) {
			free = int(i)
		}
	}
	if free >= 0 {
		g.linger[free] = lingerRec{sl: q, obj: o}
		g.lingerM.set(q)
		return
	}
	if g.nlinger < maxLinger {
		g.linger[g.nlinger] = lingerRec{sl: q, obj: o}
		g.nlinger++
		g.lingerM.set(q)
		return
	}
	s.capFull(StallLinger)
}

// lingerDrain is the state-based check's unattaching. A phased-out attachment is
// not checked (gorge attachmentSBAs skips it, CR 702.26b), and an Aura whose own
// move to the graveyard competes keeps showing its host until that move is answered.
//
//go:noinline
func (s *State) lingerDrain(k *prep) {
	g := &s.g
	var keep slotMask
	if k.phasing {
		keep = g.lingerM.and(s.phasedSlots())
	}
	for m := g.lingerM; m.any(); m = m.rest() {
		sl := m.first()
		if cd := &k.t.Cards[s.cardAt(sl)]; s.typesAt(k, sl)&kop.TAura != 0 && !cd.HasBestow && s.graveMoveCompetes(k, s.perms[sl].obj) {
			keep = keep.with(sl)
		}
	}
	for m := g.lingerM.andNot(keep); m.any(); m = m.rest() {
		s.touchSl(m.first())
	}
	n := uint8(0)
	for i := uint8(0); i < g.nlinger; i++ {
		if r := g.linger[i]; keep.has(r.sl) {
			g.linger[n] = r
			n++
		}
	}
	g.lingerM, g.nlinger = keep, n
}

// lingerObj is slot q's lingering host object; the caller checks
// lingerM.has(q) first. Cold: projections only.
//
//go:noinline
func (s *State) lingerObj(q Slot) Obj {
	for i := uint8(0); i < s.g.nlinger; i++ {
		if s.g.linger[i].sl == q {
			return s.g.linger[i].obj
		}
	}
	return NoObj
}

// lingerHost is slot q's lingering host for a projection: lingerObj, unless
// that host has since re-entered the battlefield (a flicker's return leg
// under the same object). The returned permanent is a new object that
// nothing is attached to (CR 400.7), and gorge breaks the live link at its
// entry (breakQueuedAttachmentsOnEntry) while the unattach waits for the
// check. The caller checks lingerM.has(q) first.
//
//go:noinline
func (s *State) lingerHost(q Slot) Obj {
	o := s.lingerObj(q)
	if o != NoObj && s.objs[o].zone == kop.ZoneBattlefield {
		return NoObj
	}
	return o
}

// attLKIRec is one departed attachment: the object that left (obj), the
// host it was attached to (host) and the host's ver at departure. Two
// records fill genState's pad before evSel; a third departure overwrites
// the first, and the slot scan in lkiAttached still covers an unreused
// slot.
type attLKIRec struct {
	obj  Obj
	host Obj
	ver  uint8
}

// recordAttLKI remembers the host of the attachment o leaving slot sl, so
// RefAttached still names it if o's slot is reused before the ability
// resolves (CR 608.2h). Cold: only a leaving attachment reaches it.
//
//go:noinline
func (s *State) recordAttLKI(o Obj, hostSlot Slot) {
	if !s.occ.has(hostSlot) {
		return
	}
	host := s.perms[hostSlot].obj
	hv := s.objs[host].ver
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.obj == o {
			r.host, r.ver = host, hv
			return
		}
	}
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.ver == 0 || s.objs[r.host].zone != kop.ZoneBattlefield || s.objs[r.host].ver != r.ver {
			r.obj, r.host, r.ver = o, host, hv
			return
		}
	}
	r := &s.g.attLKI[0]
	r.obj, r.host, r.ver = o, host, hv
}

// attachedToDeparted reports whether the attachment at slot q lost host o
// when o left the battlefield and has not been attached since: gorge's
// were-attached read (`AttachedTo == 0 && LastBearer == o`,
// attachedToDefinedSelector) for a chain whose earlier link moved the host
// (Fumble's bounce before its GainControl and Attach). The record lives until
// the next state-based check drains it, which is when gorge's selector reads
// it too. gorge's LastBearer outlives the drain (an Equipment that fell off a
// host still names it when that host, the same ObjID, returns and is
// targeted again); the kernel's does not, so that stale read is not modelled
// (it is CR 400.7's new object). Cold: only an AttachedTo Targeted filter
// over a departed target reaches it.
//
//go:noinline
func (s *State) attachedToDeparted(q Slot, o Obj) bool {
	return s.perms[q].attached == NoSlot && s.g.lingerM.has(q) && s.lingerObj(q) == o
}
