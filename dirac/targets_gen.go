package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Multi-object target slots (TargetSpec.Min/Max, the kicked range, the
// distinct / same / different-controller constraints). The decision
// sequence: one DkTarget pick per object (AkTarget{target}), closed by
// AkDone once the minimum is met (automatically at the maximum; a
// done-only decision when no candidate is left, as gorge asks). The first
// object sits in tgt[slot], the others in the entry's xTargets. A divided
// slot's damage (TargetSpec.Divide) is announced after the targets and
// before payment (CR 601.2d, bsDivide) and dealt as announced at
// resolution: OpDealDamageDivided.

// multiTgt is the multi-object slot being chosen.
type multiTgt struct {
	active   bool
	slot     uint8
	picked   uint8
	min, max uint8
}

// multiSlot reports whether spec t needs the multi-object path.
func multiSlot(t *kop.TargetSpec) bool {
	// TgfDecl only marks a copy declaration; TgfTrigChooser only moves the
	// ask's player (cast.go), a single-object slot's qualifier; the player
	// comparison flags only narrow a TgtPlayer slot's candidates (targetOK)
	return t.Min|t.Max != 0 || t.Divide != 0 || t.TFlags&^(kop.TgfDecl|kop.TgfTrigChooser|kop.TgfPlayerCompareOK) != 0
}

const (
	mtNext  = 0 // the slot is done: continue the build
	mtPosed = 1 // a decision is posed
	mtAbort = 2 // a trigger with no legal target was removed
)

// kickedOnlySlot reports whether a TgfKickedRange slot takes targets only
// when kicked: no range of its own (0..0, which elsewhere means one) and a
// non-zero kicked minimum (Tear Asunder's "exile target nonland permanent
// instead", announced on cast). Divine Resilience's
// slot (one unkicked, any number kicked: MinKicked 0) keeps the default.
func kickedOnlySlot(t *kop.TargetSpec) bool {
	return t.Min|t.Max == 0 && t.MinKicked != 0
}

func (s *State) multiTargetStep(k *prep, d *Decision, t *kop.TargetSpec, ti uint8) int {
	e := &s.bld().e
	mt := &s.g.mt
	if !mt.active || mt.slot != ti {
		*mt = multiTgt{active: true, slot: ti, min: 1, max: 1}
		if t.Min|t.Max != 0 {
			mt.min, mt.max = t.Min, t.Max
		}
		if t.UpTo {
			mt.min = 0
		}
		if t.Divide == kop.DivLKIP1P1 {
			s.divLKIBounds(e, mt) // ctr_divided.go
		}
		if t.TFlags&kop.TgfBoundsV != 0 {
			s.tgtBoundsV(k, t, e, mt) // tgt_variablemax.go
		}
		if t.TFlags&kop.TgfKickedRange != 0 && (e.flags&sfKicked != 0 || s.bargainElected(k, e)) {
			mt.min, mt.max = t.MinKicked, t.MaxKicked
			if mt.max == 255 {
				// "any number": gorge's TargetMax$ is the count of the
				// objects the slot can take (Divine Resilience's
				// Count$Valid Creature.YouCtrl), so picking every one
				// closes the slot with no done-only ask (pair 1473)
				var buf [MaxOptions]TCode
				mt.max = max8(uint8(s.multiCands(k, t, ti, &buf)), mt.min)
			}
		} else if t.TFlags&kop.TgfKickedRange != 0 && kickedOnlySlot(t) {
			mt.min, mt.max = 0, 0 // Tear Asunder's kicked slot, unkicked: no target
		}
		if slotNoneUnkicked(t) && e.flags&sfKicked == 0 && !s.bargainElected(k, e) {
			mt.min, mt.max = 0, 0 // none unless kicked (mana_restrictedsource.go)
		}
		if t.TFlags&kop.TgfGiftRange != 0 && !s.giftSlotStart(k, t, ti) {
			return mtAbort
		}
		if t.TFlags&kop.TgfSameController != 0 {
			// gorge bounds a same-controller slot by its largest group
			// (Soul-Shackled Zombie: "up to two from a single graveyard"
			// with one card in each closes after the first pick)
			var buf [MaxOptions]TCode
			n := s.multiCands(k, t, ti, &buf)
			var per [MaxSeats]uint8
			for i := 0; i < n; i++ {
				if c := buf[i]; isObjTarget(c) {
					per[SeatIx(s.ctrlOfObj(Obj(c)))]++
				}
			}
			if g := seatMax(&per); g < mt.max && g > 0 {
				mt.max = max8(g, mt.min)
			}
		}
		if t.TFlags&kop.TgfDiffController != 0 {
			if s.diffCtrlCap(k, t, ti, mt) < mt.min { // zone_untilleaves.go
				mt.max = 0 // fewer controllers than the minimum: no legal choice
			}
		}
		if t.TFlags&(kop.TgfSharedCrType|kop.TgfDistinctNames) != 0 {
			s.tgtSetBounds(k, t, e, mt) // tgt_setconstraints.go
		}
	}
	var buf [MaxOptions]TCode
	n := 0
	if mt.picked < mt.max {
		n = s.multiCands(k, t, ti, &buf)
		if e.kind == seSpell {
			// gorge prices each candidate as the sole selection and drops the
			// ones the cast cannot pay for, for a multi-target ask as for a
			// single one (cast_pricing.go affordableTargetCandidates)
			all := n
			n = s.affordableTargets(k, &buf, n)
			if n < all && n < int(mt.min) && mt.picked == 0 && (k.gen.anyRaiseLife || k.gen.anyRaise) {
				// the candidates the cast can pay for fall short of the slot's
				// minimum: gorge's ask aborts the cast ("cast aborted: no legal
				// target"; a life raise on a target, cost_raiselife.go)
				mt.active = false
				s.reverseCast(k)
				return mtAbort
			}
		}
		if k.gen.anyUniqLater {
			n = s.uniqueChainViable(k, ti, &buf, n) // tgt_uniquechain.go
		}
	}
	if n == 0 && mt.picked < mt.max && mt.picked >= mt.min && mt.picked > 0 {
		// below the maximum with nothing left to target: gorge still asks,
		// done only (with nothing to target at all a zero-minimum slot asks
		// nothing, a spell's as a trigger's: gorge continueCast)
		d.Kind, d.Player = DkTarget, e.ctrl
		d.add(MakeAction(AkDone, 0, 0, 0))
		return mtPosed
	}
	if n == 0 || mt.picked >= mt.max {
		if mt.picked < mt.min && mt.picked == 0 {
			if e.kind == seTrigger {
				mt.active = false
				s.sp--
				s.ctl = ctlPlaceTriggers
				return mtAbort
			}
			if e.kind == seSpell && t.TFlags&kop.TgfDistinct != 0 && ti > 0 {
				// a mandatory "another target" slot the earlier picks left
				// empty: gorge's ask aborts the cast ("cast aborted: no
				// legal target", CR 601.2c / 733.1), as the single-object
				// path reverses it (Band Together naming both creatures)
				mt.active = false
				s.reverseCast(k)
				return mtAbort
			}
		}
		return s.finishSlot(k, d, t)
	}
	d.Kind, d.Player = DkTarget, e.ctrl
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkTarget, 0, buf[i], 0))
	}
	doneOK := mt.picked >= mt.min
	if doneOK && mt.picked == 0 && mt.min == 0 && e.kind == seSpell && e.flags&(sfMayPlay|sfFlashback) == 0 {
		doneOK = s.emptyTargetSetPayable(k)
	}
	if doneOK {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return mtPosed
}

// emptyTargetSetPayable reports whether the "up to" empty answer leaves the
// spell payable. A target-dependent price is fixed by the targets chosen (CR
// 601.2c before 601.2f), so the empty answer can make the cost one no source
// can pay, and gorge withholds it exactly as it withholds an unaffordable
// candidate (targetAskWithholdsEmptyTargetSet, which keys on every cost static
// that prices by target) rather than exposing a completion that only reverses
// the cast. The dependence is read from the composition itself: the cast
// card's own ValidTarget$ static or a life raise (spellTgtDep), or any
// battlefield reduction the cost build applies for some candidate target but
// not for none.
func (s *State) emptyTargetSetPayable(k *prep) bool {
	e := &s.bld().e
	b := s.bld()
	fixed, was := b.fixedGen, s.g.noTgtRed
	s.g.noTgtRed = false
	full := s.buildCost(k)
	s.g.noTgtRed = true
	zero := s.buildCost(k)
	s.g.noTgtRed, b.fixedGen = was, fixed
	if full == zero && !s.spellTgtDep(k, e.ctrl, e.card, e.obj) {
		return true
	}
	var av avail
	s.availFor(k, e.ctrl, &av)
	return av.affords(&zero, pipMaskOf(&zero), 0)
}

// finishSlot ends the slot's picks (a divided slot's damage is announced
// after the targets, before payment: bsDivide).
func (s *State) finishSlot(k *prep, d *Decision, t *kop.TargetSpec) int {
	if k.gen != nil && k.gen.crimeOn && s.bld().e.kind != seSpell {
		s.crimeAskDone(k) // the committed window's TargetsChosen (act_commitcrime.go)
	}
	s.g.mt.active = false
	s.bld().ti++
	return mtNext
}

// multiCands lists the slot's remaining candidates.
func (s *State) multiCands(k *prep, t *kop.TargetSpec, ti uint8, out *[MaxOptions]TCode) int {
	e := &s.bld().e
	var all [MaxOptions]TCode
	n := s.targetCands(k, t, e.ctrl, s.srcSlot(), e.obj, &all)
	mt := &s.g.mt
	var per [MaxSeats]uint8
	if mt.picked == 0 && mt.min > 1 && t.TFlags&kop.TgfSameController != 0 {
		// a same-controller slot's first pick comes from a controller
		// with enough for the whole slot (gorge: Pestilent Cauldron's
		// four from a single graveyard offers no card of a graveyard
		// with three)
		for i := 0; i < n; i++ {
			if c := all[i]; c != NoTarget && isObjTarget(c) {
				per[SeatIx(s.ctrlOfObj(Obj(c)))]++
			}
		}
	}
	m := 0
	for i := 0; i < n; i++ {
		c := all[i]
		if c == NoTarget || s.pickedInSlot(e, ti, c) {
			continue
		}
		if per[0]|per[1] != 0 && isObjTarget(c) && per[SeatIx(s.ctrlOfObj(Obj(c)))] < mt.min {
			continue
		}
		if t.TFlags&kop.TgfDistinct != 0 && s.pickedBefore(e, ti, c) {
			continue
		}
		if mt.picked > 0 && isObjTarget(c) && t.TFlags&(kop.TgfSameController|kop.TgfDiffController) != 0 {
			first := e.tgt[ti]
			if isObjTarget(first) {
				same := s.ctrlOfObj(Obj(c)) == s.ctrlOfObj(Obj(first))
				if t.TFlags&kop.TgfSameController != 0 && !same || t.TFlags&kop.TgfDiffController != 0 && same {
					continue
				}
			}
		}
		if t.TFlags&tgtSetMask != 0 && !s.tgtSetAdmit(k, t, e, ti, c, all[:n]) {
			continue
		}
		out[m] = c
		m++
	}
	return m
}

func (s *State) pickedInSlot(e *stackEntry, ti uint8, c TCode) bool {
	if s.g.mt.picked > 0 && e.tgt[ti] == c {
		return true
	}
	for i := 0; i < int(s.xtOf(e).n); i++ {
		if s.xtOf(e).slot[i] == ti && s.xtOf(e).code[i] == c {
			return true
		}
	}
	return false
}

func (s *State) pickedBefore(e *stackEntry, ti uint8, c TCode) bool {
	for i := uint8(0); i < ti; i++ {
		if e.tgt[i] == c {
			return true
		}
	}
	for i := 0; i < int(s.xtOf(e).n); i++ {
		if s.xtOf(e).slot[i] < ti && s.xtOf(e).code[i] == c {
			return true
		}
	}
	return false
}

// applyMultiTarget applies a pick of the multi-object slot in progress.
func (s *State) applyMultiTarget(k *prep, a Action) {
	e := &s.bld().e
	mt := &s.g.mt
	switch a.Kind() {
	case AkTarget:
		c := a.Target()
		var ver uint8
		if isObjTarget(c) {
			ver = s.objs[c].ver
		} else if isStackTarget(c) {
			ver = s.stackTgtSid(c)
		}
		if k.gen != nil && k.gen.crimeOn {
			// a multi-pick window's pick: a spell's is judged here (crimePick),
			// an ability's with the slot's whole set at completion (finishSlot)
			s.crimePick(k, c, false)
		}
		if mt.picked == 0 {
			e.tgt[mt.slot] = c
			e.tver[mt.slot] = ver
			if mt.slot+1 > e.nt {
				e.nt = mt.slot + 1
			}
		} else if x := s.xtMut(e); x == nil {
			return
		} else if x.n < maxXTgt {
			x.slot[x.n] = mt.slot
			x.code[x.n] = c
			x.ver[x.n] = ver
			x.n++
		} else {
			s.capFull(StallXTgt)
		}
		mt.picked++
	case AkDone:
		mt.picked = max8(mt.picked, 0)
		mt.max = mt.picked // no more picks: the next step finishes the slot
	default:
		panic("kernel: bad multi-target action")
	}
}

// hasTargetsMin: a slot needing two or more objects has enough candidates
// (with different controllers when it asks for that, or enough under one
// controller when it asks for the same: Pestilent Cauldron's "four target
// cards from a single graveyard", gorge castTargetsAvailable).
func (s *State) hasTargetsMin(k *prep, t *kop.TargetSpec, p uint8, src Slot, self Obj) bool {
	var buf [MaxOptions]TCode
	n := s.targetCands(k, t, p, src, self, &buf)
	if n < int(t.Min) {
		return false
	}
	if t.TFlags&kop.TgfSharedCrType != 0 {
		// returns before the same/different-controller checks below: kgen
		// refuses a shared-type slot beside SameCtrl/DiffCtrl, so they never meet.
		return s.tgtSetCap(k, t, buf[:n]) >= t.Min // tgt_setconstraints.go
	}
	if t.TFlags&kop.TgfSameController != 0 {
		var per [MaxSeats]uint8
		for i := 0; i < n; i++ {
			if isObjTarget(buf[i]) {
				per[SeatIx(s.ctrlOfObj(Obj(buf[i])))]++
			}
		}
		return seatMax(&per) >= t.Min
	}
	if t.TFlags&kop.TgfDiffController != 0 {
		var seen [MaxSeats]bool
		for i := 0; i < n; i++ {
			if isObjTarget(buf[i]) {
				seen[SeatIx(s.ctrlOfObj(Obj(buf[i])))] = true
			}
		}
		return seatCount(&seen) >= 2
	}
	return true
}
