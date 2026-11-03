package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCloneCopy, DB$ Clone on the battlefield (gorge
// effects/clone.go effClone; CR 707.2, 707.9b, 613.2a, 611.2c, 514.2, 400.7).
//
// A becomer (the source, a target slot, or every permanent of a Valid set)
// becomes a copy of the template permanent: the steps of becomeCopyEnter
// (unsubscribe the old card, ovCopy/copyOf, subscribe the new card, the
// template's and the op's copy exceptions).
//
// Layer 1 holds a permanent's copy markers (CR 613.2a), and gorge settles
// them when an until-end-of-turn one expires (settleExpiredClones): the
// permanent goes back to its printed card, then is re-based onto the newest
// permanent marker it still holds, copying that marker's SOURCE as it is at
// that moment (a source that is itself being settled in the same cleanup step
// may or may not have been yet: the order is the first until-end-of-turn
// marker's). Two record kinds model it:
//
//   - fxCopyEOT: the permanent holds until-end-of-turn copies. One record,
//     pushed with the first; the pool order is gorge's settle order.
//   - fxCopyPerm: its newest permanent copy marker: r.p the source object,
//     r.idx the op's exception override (index+1). Pushed only when a table
//     has an ExtCloneCopy op (genPrep.anyCloneCopy), by it and by an enters
//     copy (becomeCopyEnter); one per permanent.
//
// The copy exceptions of the copy on top are fxCopyExc records, replaced with
// every copy. cloneCleanup runs the settle. A permanent that leaves the
// battlefield loses all its records through clearFx (CR 400.7).
//
// A token copy of a permanent that is itself a copy is minted as gorge's
// copyTokenBasis mints it (events/apply_copy.go): on that permanent's PRINTED
// card, its copied characteristics a layer-1 override (cloneTokenBasis), so it
// projects with the printed card's name.
//
// An Aegis bearer, a permanent copying itself, or such a minted token becoming
// an until-end-of-turn copy (gorge's settle would put it back on the printed
// card it was minted on, not on the copy it is: see the report) stalls
// StallCloneLayer.

// DebugCloneCopyNoRevert is the mutation switch of TestClonecopyMutation: the
// cleanup step never ends an until-end-of-turn copy.
var DebugCloneCopyNoRevert bool

// DebugCloneCopyNoRebase is the mutation switch of TestClonecopyMutation: the
// cleanup step puts a permanent back on its printed card and never re-bases
// it on the permanent copy marker it still holds (the layer under the
// until-end-of-turn one).
var DebugCloneCopyNoRebase bool

// CloneCopyStats counts ExtCloneCopy events (TestClonecopyLockstep).
type CloneCopyStats struct {
	EOT      uint32 // until-end-of-turn copies made
	Perm     uint32 // permanent copies made
	Reverted uint32 // until-end-of-turn copies ended at cleanup
	Multi    uint32 // resolutions that copied onto two or more becomers
	Asked    uint32 // Optional$ elections posed
	Moved    uint32 // optional triggers asked although their source had become a copy while they waited
	Minted   uint32 // token copies of a copy minted on its printed card (cloneTokenBasis)
	Stacked  uint32 // copies made onto a becomer that already held a copy layer (a copy marker or an enters copy)
	Rebased  uint32 // cleanup settles that re-based a permanent on the permanent copy layer under the ended one
}

// DebugCloneCopyStats, when set, receives the counts.
var DebugCloneCopyStats *CloneCopyStats

// StallCloneLayer: a copy onto an Aegis bearer (its own layer-1 bookkeeping),
// a permanent copying itself, or an until-end-of-turn copy onto a token minted
// as a copy of a copy (cloneMinted).
const StallCloneLayer = 141

func init() { stallNames[StallCloneLayer] = "clonelayer" }

// ChCloneCopyPick is gorge's clone_choice KChoose over the battlefield
// permanents matching the Choices$ filter: source = the template picked.
const ChCloneCopyPick ChoiceKind = 243

// cloneTmpl is the template's slot: the Choices$ pick or the target slot;
// NoSlot when it is not (any longer) on the battlefield.
func (s *State) cloneTmpl(k *prep, in *kop.Instr) Slot {
	b := in.B.N
	if b&kop.CloneTmplPick != 0 {
		if s.g.nchosen == 0 {
			return NoSlot
		}
		ob := &s.objs[s.g.chosen[0]]
		if ob.zone != kop.ZoneBattlefield {
			return NoSlot
		}
		return ob.slot
	}
	return s.objRef(k, kop.RefTarget0+kop.Ref(b>>kop.CloneTmplShift&3))
}

// cloneBecomers lists the becomers on the battlefield in OpDamageAll's slot
// order (seat 0 first, entry order), the template dropped when asked.
func (s *State) cloneBecomers(k *prep, in *kop.Instr, tmpl Slot, ord *[MaxPerm]Slot) int {
	if in.Obj != kop.RefNone {
		sl := s.objRef(k, in.Obj)
		if sl == NoSlot {
			return 0
		}
		ord[0] = sl
		return 1
	}
	n := s.sweepHits(k, in.F, ord)
	if in.B.N&kop.CloneExclTmpl == 0 {
		return n
	}
	m := 0
	for i := 0; i < n; i++ {
		if ord[i] != tmpl {
			ord[m] = ord[i]
			m++
		}
	}
	return m
}

// cloneAsks is MayGuardCloneCopy: the next op has a live template and a
// becomer (gorge's pairs, computed before the Optional$ election).
func (s *State) cloneAsks(k *prep, next *kop.Instr) bool {
	if next.Ext() != kop.ExtCloneCopy {
		return false
	}
	tmpl := s.cloneTmpl(k, next)
	if tmpl == NoSlot {
		return false
	}
	var ord [MaxPerm]Slot
	if s.cloneBecomers(k, next, tmpl, &ord) == 0 {
		return false
	}
	if DebugCloneCopyStats != nil {
		DebugCloneCopyStats.Asked++
	}
	return true
}

// cloneLayered: slot sl holds a copy this op cannot stack another on.
func (s *State) cloneLayered(k *prep, sl Slot) bool {
	return k.gen.anyAegis && s.aegisLive(sl) > 0
}

// cloneLayerPrep clears slot sl for a new copy and returns the keywords its
// current copy exceptions grant: the first until-end-of-turn copy pushes the
// settle record, and the exceptions of the copy it replaces go. False when
// the effect pool is full.
//
//go:noinline
func (s *State) cloneLayerPrep(k *prep, sl Slot, eot bool) (old kop.KwMask, ok bool) {
	has := s.fxM.has(sl)
	if has {
		old = s.copyExcKw(k, sl)
	}
	if eot {
		anchored := false
		for i := 0; has && i < int(s.g.nfx); i++ {
			anchored = anchored || s.g.fx[i].sl == sl && s.g.fx[i].kind == fxCopyEOT
		}
		if !anchored {
			if int(s.g.nfx) >= maxFx {
				s.stall(StallTables)
				return 0, false
			}
			s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopyEOT}
			s.g.nfx++
			s.fxM.set(sl)
		}
	}
	if has {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == sl && r.kind == fxCopyExc })
	}
	return old, true
}

// clonePermNote records that slot sl's newest permanent copy marker copies
// object src, with the exception override idx (index+1, 0 none).
//
//go:noinline
func (s *State) clonePermNote(sl Slot, src Obj, idx uint8) {
	if s.fxM.has(sl) {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == sl && r.kind == fxCopyPerm })
	}
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopyPerm, r: ovRec{idx: idx, p: int16(src)}}
	s.g.nfx++
	s.fxM.set(sl)
}

// cloneCopy is ExtCloneCopy's copy phase.
//
//go:noinline
func (s *State) cloneCopy(k *prep, in *kop.Instr) {
	tmpl := s.cloneTmpl(k, in)
	if tmpl == NoSlot {
		return // the template left (or was never chosen): nothing happens
	}
	var ord [MaxPerm]Slot
	n := s.cloneBecomers(k, in, tmpl, &ord)
	if n == 0 {
		return
	}
	eot := in.B.N&kop.CloneEOT != 0
	c := s.cardAt(tmpl)
	made := 0
	for i := 0; i < n; i++ {
		sl := ord[i]
		if sl == tmpl || s.cloneLayered(k, sl) || eot && s.cloneMinted(sl) {
			s.capFull(StallCloneLayer)
			continue
		}
		p := &s.perms[sl]
		if DebugCloneCopyStats != nil && p.ovr.flags&ovCopy != 0 {
			DebugCloneCopyStats.Stacked++
		}
		s.touchSl(sl)
		old, ok := s.cloneLayerPrep(k, sl, eot)
		if !ok {
			return
		}
		s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
		p.ovr.flags |= ovCopy
		p.ovr.copyOf = c
		s.subscribe(sl, &k.cards[c], true)
		s.copyExcAdd(k, sl, tmpl, uint8(in.C))
		p.ovr.kw = p.ovr.kw&^old | s.copyExcKw(k, sl)
		if !eot {
			s.clonePermNote(sl, s.perms[tmpl].obj, uint8(in.C))
		}
		made++
	}
	if made == 0 {
		return
	}
	s.refreshTy(k)
	if st := DebugCloneCopyStats; st != nil {
		if eot {
			st.EOT += uint32(made)
		} else {
			st.Perm += uint32(made)
		}
		if made >= 2 {
			st.Multi++
		}
	}
}

// clonePick is ExtCloneCopy's pick phase (Choices$): the controller picks the
// template among the battlefield permanents matching F; nothing is asked
// with no candidate.
func (s *State) clonePick(k *prep, in *kop.Instr) bool {
	s.g.nchosen = 0
	if s.cloneCands(k, in, nil) == 0 {
		return true
	}
	s.res.ask, s.res.askP = raHideaway, s.res.e.ctrl
	return false
}

// cloneCands counts (and lists in d) the pick's candidates.
func (s *State) cloneCands(k *prep, in *kop.Instr, d *Decision) int {
	src := s.objRef(k, kop.RefSelf)
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPermGen(k, in.F, sl, s.res.e.ctrl, src, layerFull) {
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			}
		}
	}
	return n
}

// cloneCopyOptions lists the pick (askOptionsHide's other half); gorge's
// clone_choice KChoose.
func (s *State) cloneCopyOptions(k *prep, d *Decision) {
	d.Sub = ChCloneCopyPick
	s.cloneCands(k, &s.prog(k)[s.res.pc], d)
}

// cloneApply takes the pick.
func (s *State) cloneApply(k *prep, a Action) {
	s.res.ask = raNone
	s.g.chosen[0] = Obj(a.Source())
	s.g.nchosen = 1
	s.continueRes(k, 0)
}

// cloneCleanup settles the permanents that hold until-end-of-turn copies at
// the cleanup step (CR 514.2; gorge settleExpiredClones), in record order: each
// goes back to its printed card with no copy exceptions, then, if it holds a
// permanent copy marker, becomes a copy of that marker's source as the source
// is now.
//
//go:noinline
func (s *State) cloneCleanup(k *prep) {
	if DebugCloneCopyNoRevert {
		return
	}
	any := false
	for {
		i := 0
		for i < int(s.g.nfx) && s.g.fx[i].kind != fxCopyEOT {
			i++
		}
		if i == int(s.g.nfx) {
			break
		}
		sl := s.g.fx[i].sl
		old := s.copyExcKw(k, sl)
		s.dropFx(func(i int) bool {
			x := &s.g.fx[i]
			return x.sl == sl && (x.kind == fxCopyEOT || x.kind == fxCopyExc)
		})
		p := &s.perms[sl]
		s.touchSl(sl)
		s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
		p.ovr.flags &^= ovCopy
		p.ovr.copyOf = 0
		for j := 0; j < int(s.g.nfx); j++ {
			x := &s.g.fx[j]
			if x.sl != sl || x.kind != fxCopyPerm || DebugCloneCopyNoRebase {
				continue
			}
			// an object id is never reused (newToken); a source off the
			// battlefield is copied as its printed card, as gorge's
			// CopiableFace is once Move cleared its CopyFace (a ceased
			// token: its token card)
			src, idx := Obj(x.r.p), x.r.idx
			ob := &s.objs[src]
			from, c := NoSlot, ob.card
			if ob.zone == kop.ZoneBattlefield {
				from = ob.slot
				c = s.cardAt(from)
			}
			p.ovr.flags |= ovCopy
			p.ovr.copyOf = c
			s.copyExcAdd(k, sl, from, idx)
			if DebugCloneCopyStats != nil {
				DebugCloneCopyStats.Rebased++
			}
			break
		}
		s.subscribe(sl, &k.cards[s.cardAt(sl)], true)
		p.ovr.kw = p.ovr.kw&^old | s.copyExcKw(k, sl)
		any = true
		if DebugCloneCopyStats != nil {
			DebugCloneCopyStats.Reverted++
		}
	}
	if any {
		s.refreshTy(k)
	}
}

// prepCloneCopy sets anyCloneCopy.
func prepCloneCopy(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtCloneCopy {
					g.anyCloneCopy = true
					return
				}
			}
		}
	}
}

// cloneTrigMoved: the trigger on the stack was put by a source whose face is
// no longer its card's (it became a copy of something else, or stopped being
// one), and gorge can no longer find it. The ability keeps its own text (CR 113.7a),
// so gorge looks for it on the live face and then, while the source is a copy, on
// its printed card: a trigger found there is asked, one found nowhere (the source
// stopped being a copy, or was a copy of another card when it triggered) is not.
//
//go:noinline
func (s *State) cloneTrigMoved(e *stackEntry) bool {
	if e.obj == NoObj {
		return false
	}
	ob := &s.objs[e.obj]
	cur := ob.card
	if ob.zone == kop.ZoneBattlefield {
		cur = s.cardAt(ob.slot)
		if cur != e.card && s.perms[ob.slot].ovr.flags&ovCopy != 0 && s.perms[ob.slot].card == e.card {
			if DebugCloneCopyStats != nil {
				DebugCloneCopyStats.Moved++
			}
			return false
		}
	}
	return cur != e.card
}

// cloneTokenBasis mints copy token t (at sl, entered as a copy of the copying
// permanent at tmpl) as gorge copyTokenBasis does: the card is tmpl's printed
// card, the copy it entered as a layer-1 override. Its characteristics and
// subscriptions are unchanged (cardAt is the same card).
//
//go:noinline
func (s *State) cloneTokenBasis(sl Slot, t Obj, tmpl Slot) {
	p := &s.perms[sl]
	p.ovr.flags |= ovCopy
	p.ovr.copyOf = p.card
	p.card = s.perms[tmpl].card
	s.objs[t].card = p.card
	if DebugCloneCopyStats != nil {
		DebugCloneCopyStats.Minted++
	}
}

// cloneMinted: slot sl is a copy token whose copy is still its minted basis
// (cloneTokenBasis): no CloneCopy marker on it.
func (s *State) cloneMinted(sl Slot) bool {
	ob := &s.objs[s.perms[sl].obj]
	if ob.of&ofToken == 0 || ob.of&ofCopy == 0 || s.perms[sl].ovr.flags&ovCopy == 0 {
		return false
	}
	for i := 0; s.fxM.has(sl) && i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && (r.kind == fxCopyPerm || r.kind == fxCopyEOT) {
			return false
		}
	}
	return true
}
