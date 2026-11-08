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
// copy layer it still holds (copyTop: a permanent marker or a live Aegis
// copy), restoring the card that marker locked when it landed (CR 707.2b;
// gorge's state.CloneLayer), never re-reading its source. Two record kinds
// model the clone layers:
//
//   - fxCopyEOT: the permanent holds until-end-of-turn copies. One record,
//     pushed with the first (the pool order is gorge's settle order), its
//     card and timestamp those of the NEWEST clone. A later clone updates
//     the record in place.
//   - fxCopyPerm: its newest permanent copy marker: r.p the source object,
//     r.src the card it showed when the copy landed, r.idx the op's
//     exception override (index+1), r.ts when it landed.
//     Pushed only when a table has an ExtCloneCopy op
//     (genPrep.anyCloneCopy), by it and by an enters copy
//     (becomeCopyEnter); one per permanent.
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
// A token copy keeps its own copiable values (CR 707.2, 707.9b): the first
// until-end-of-turn copy onto it holds the copy it was minted as and its own
// copy exceptions aside (cloneHold), and the cleanup step puts them back, as
// gorge restores state.Object.TokenBasis.
//
// A permanent copying itself stalls StallCloneLayer. A copy onto an Aegis
// bearer is the Aegis layer covered by a newer one: copy_aegisunder.go keeps
// every layer's timestamp, and the cleanup step gives the newest one left
// back.

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
	Held     uint32 // until-end-of-turn copies onto a token copy that held its own copy aside (cloneHold)
	Restored uint32 // cleanup settles that put a token copy back on a held copy of another card than its printed one
}

// DebugCloneCopyNoHold is the mutation switch of TestCloneTokenBasisMutation:
// a token copy's own copy is never held aside, so the cleanup step puts it
// back on the printed card it was minted on.
var DebugCloneCopyNoHold bool

// DebugCloneCopyStats, when set, receives the counts.
var DebugCloneCopyStats *CloneCopyStats

// StallCloneLayer: a permanent copying itself.
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

// cloneLayerPrep clears slot sl for a new copy and returns the keywords its
// current copy exceptions grant: the until-end-of-turn copy marker records
// the new layer's card and timestamp (the newest clone shows while it lasts,
// copyTop), and the exceptions of the copy it replaces go. False when the
// effect pool is full.
//
//go:noinline
func (s *State) cloneLayerPrep(k *prep, sl Slot, eot bool, c CardID, src Obj, idx uint8, ts uint16) (old kop.KwMask, ok bool) {
	has := s.fxM.has(sl)
	if has {
		old = s.copyExcKw(k, sl)
	}
	if eot {
		i := -1
		for j := 0; has && j < int(s.g.nfx); j++ {
			if s.g.fx[j].sl == sl && s.g.fx[j].kind == fxCopyEOT {
				i = j
				break
			}
		}
		if i < 0 {
			if int(s.g.nfx) >= maxFx {
				s.stall(StallTables)
				return 0, false
			}
			i = int(s.g.nfx)
			s.g.fx[i] = fxRec{sl: sl, kind: fxCopyEOT}
			s.g.nfx++
			s.fxM.set(sl)
		}
		s.g.fx[i].r = ovRec{idx: idx, src: c, p: int16(src), ts: ts}
	}
	if has {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == sl && r.kind == fxCopyExc })
	}
	return old, true
}

// clonePermNote records that slot sl's newest permanent copy marker copies
// object src, with the exception override idx (index+1, 0 none), as the
// layer timestamp ts. c is the card the copy showed when it was applied: the
// layer's copiable values are locked then (CR 707.2b), so a later re-base
// restores c and never reads src again.
//
//go:noinline
func (s *State) clonePermNote(sl Slot, src Obj, c CardID, idx uint8, ts uint16) {
	if s.fxM.has(sl) {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == sl && r.kind == fxCopyPerm })
	}
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopyPerm, r: ovRec{idx: idx, src: c, p: int16(src), ts: ts}}
	s.g.nfx++
	s.fxM.set(sl)
}

// cloneCopy is ExtCloneCopy's copy phase.
//
//go:noinline
func (s *State) cloneCopy(k *prep, in *kop.Instr) {
	var tmpl Slot
	var c CardID
	if in.B.N&kop.CloneBatch != 0 {
		// a batch pick is a card in exile, copied as its printed card (gorge
		// checks the template is still in the chosen zone, clone.go)
		if s.g.nchosen == 0 || s.objs[s.g.chosen[0]].zone != kop.ZoneExile {
			return
		}
		tmpl, c = NoSlot, s.objs[s.g.chosen[0]].card
	} else if tmpl = s.cloneTmpl(k, in); tmpl == NoSlot {
		return // the template left (or was never chosen): nothing happens
	} else {
		c = s.cardAt(tmpl)
	}
	var ord [MaxPerm]Slot
	n := s.cloneBecomers(k, in, tmpl, &ord)
	if n == 0 {
		return
	}
	eot := in.B.N&kop.CloneEOT != 0
	made := 0
	for i := 0; i < n; i++ {
		sl := ord[i]
		if sl == tmpl && eot && in.C == 0 && in.Obj == kop.RefNone && in.B.N&kop.CloneExclTmpl == 0 {
			continue // a Valid set holding its template: a copy of itself changes nothing (CR 707.2)
		}
		if sl == tmpl {
			s.capFull(StallCloneLayer)
			continue
		}
		p := &s.perms[sl]
		ts := s.tick()
		if DebugCloneCopyStats != nil && p.ovr.flags&ovCopy != 0 {
			DebugCloneCopyStats.Stacked++
		}
		s.touchSl(sl)
		s.onceBasisBegin(sl)
		held, ok := kop.KwMask(0), true
		if eot {
			held, ok = s.cloneHold(k, sl)
		}
		if !ok {
			return
		}
		src := NoObj
		if tmpl != NoSlot {
			src = s.perms[tmpl].obj
		}
		old, ok := s.cloneLayerPrep(k, sl, eot, c, src, uint8(in.C), ts)
		if !ok {
			return
		}
		old |= held
		s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
		p.ovr.flags |= ovCopy
		p.ovr.copyOf = c
		s.subscribe(sl, &k.cards[c], true)
		s.copyExcAdd(k, sl, tmpl, uint8(in.C))
		p.ovr.kw = p.ovr.kw&^old | s.copyExcKw(k, sl)
		if !eot {
			s.clonePermNote(sl, s.perms[tmpl].obj, c, uint8(in.C), ts)
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
	if in.B.N&kop.CloneBatch != 0 {
		return s.cloneBatchCands(k, in, d) // trig_batchmembers.go
	}
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
	if s.prog(k)[s.res.pc].B.N&kop.CloneBatch != 0 {
		d.Sub = ChCloneBatchPick // gorge's card pick over the zone's cards
	}
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
// the cleanup step (CR 514.2; gorge settleExpiredClones), in record order:
// the ended copies go, and the permanent is left on the newest copy layer it
// still holds (copyTop): a live Aegis copy, or a permanent copy marker copied
// as its source is now. With no layer left it goes back to its printed card,
// or to a token copy's own basis.
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
		// The layer the ended copies leave: a live Aegis copy or the
		// permanent marker, whichever is newer (the ended layer itself is
		// already gone from the pool, so copyTop does not see it).
		l := s.copyTop(sl, NoSlot)
		if l.kind == copyPerm && DebugCloneCopyNoRebase {
			l = copyLayer{}
		}
		if l.kind == copyNone {
			// the floor: the copy the Aegis stack covered that has no
			// marker of its own
			if c, ok := s.aegisUnderTake(sl); ok {
				l = copyLayer{kind: copyBase, card: c}
			}
		}
		p := &s.perms[sl]
		s.touchSl(sl)
		s.subscribe(sl, &k.cards[s.cardAt(sl)], false)
		p.ovr.flags &^= ovCopy
		p.ovr.copyOf = 0
		if l.kind != copyNone {
			p.ovr.flags |= ovCopy
			p.ovr.copyOf = l.card
		}
		s.onceBasisEnd(sl, l.kind != copyNone)
		if l.kind == copyNone {
			s.cloneRestore(sl)
		} else if l.kind == copyPerm {
			s.cloneRestore(sl) // the marker wins: drops the token basis records
			s.copyExcAdd(k, sl, s.copySrcSlot(l.src), l.idx)
			if DebugCloneCopyStats != nil {
				DebugCloneCopyStats.Rebased++
			}
		} else if l.kind == copyBase {
			s.aegisRestore(sl) // the covered copy's exceptions, held aside
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

// copySrcSlot is the slot an object's copiable values are read from, NoSlot
// when it is off the battlefield (its printed card).
func (s *State) copySrcSlot(o Obj) Slot {
	if int(o) >= MaxObj {
		return NoSlot
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		return ob.slot
	}
	return NoSlot
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
func (s *State) cloneTrigMoved(k *prep, e *stackEntry) bool {
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
	if cur != e.card {
		// a two-faced card whose line is printed on its other face (a back-face
		// "when this dies" that triggered from the look-back board): gorge's
		// owning-line scan finds it there, off the battlefield as on it
		// (trigger_inactive_face.go, CR 712.8a)
		cd := &k.t.Cards[ob.card]
		if (cd.FaceMode == kop.FaceTransform || cd.FaceMode == kop.FaceModal) && len(cd.Faces) == 2 &&
			(cd.Faces[0] == e.card || cd.Faces[1] == e.card) {
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

// cloneHold holds a token copy's own copy aside as the first copy layer
// lands on it (no copy marker on it yet): the copy it was minted as (an
// fxCopyMint record) and its copy exceptions (fxCopyExc records become
// fxCopyHeld). It returns the keywords the held exceptions granted; false
// when the effect pool is full. gorge saves state.Object.TokenBasis.
//
//go:noinline
func (s *State) cloneHold(k *prep, sl Slot) (kw kop.KwMask, ok bool) {
	p := &s.perms[sl]
	ob := &s.objs[p.obj]
	if DebugCloneCopyNoHold || ob.of&(ofToken|ofCopy) != ofToken|ofCopy || s.cloneBased(sl) {
		return 0, true
	}
	if p.ovr.flags&ovCopy != 0 {
		if int(s.g.nfx) >= maxFx {
			s.stall(StallTables)
			return 0, false
		}
		s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxCopyMint, r: ovRec{src: p.ovr.copyOf}}
		s.g.nfx++
		s.fxM.set(sl)
	}
	for i := 0; s.fxM.has(sl) && i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			kw |= k.t.Overrides[r.r.idx-1].AddKw
			r.kind = fxCopyHeld
		}
	}
	if DebugCloneCopyStats != nil {
		DebugCloneCopyStats.Held++
	}
	return kw, true
}

// cloneRestore puts a settling token copy back on the copy cloneHold held
// aside, its exceptions live again; with a permanent copy marker left, the
// re-base replaces them and the held records go.
//
//go:noinline
func (s *State) cloneRestore(sl Slot) {
	if !s.fxM.has(sl) {
		return
	}
	perm := false
	for i := 0; i < int(s.g.nfx); i++ {
		perm = perm || s.g.fx[i].sl == sl && s.g.fx[i].kind == fxCopyPerm
	}
	p := &s.perms[sl]
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != sl || perm {
			continue
		}
		switch r.kind {
		case fxCopyHeld:
			r.kind = fxCopyExc
		case fxCopyMint:
			p.ovr.flags |= ovCopy
			p.ovr.copyOf = r.r.src
			if DebugCloneCopyStats != nil && r.r.src != p.card {
				DebugCloneCopyStats.Restored++
			}
		}
	}
	s.dropFx(func(i int) bool {
		r := &s.g.fx[i]
		return r.sl == sl && (r.kind == fxCopyMint || r.kind == fxCopyHeld)
	})
}

// cloneBased: slot sl holds a copy marker (an until-end-of-turn or permanent
// one), so its abilities are a copy layer's.
func (s *State) cloneBased(sl Slot) bool {
	for i := 0; s.fxM.has(sl) && i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && (r.kind == fxCopyPerm || r.kind == fxCopyEOT) {
			return true
		}
	}
	return false
}

// onceBasisBegin: a new copy makes the permanent's abilities new ones that
// nothing has activated (CR 613.1a; gorge activationUsedSince counts the
// events since the newest ClonePermanent), so its "activate only once"
// mark (Power-up, CR 702.193a) is clear. The first copy layer keeps the
// printed abilities' mark aside, which returns once the layer ends.
func (s *State) onceBasisBegin(sl Slot) {
	p := &s.perms[sl]
	if p.flags&pfOnceGame == 0 {
		return
	}
	p.flags &^= pfOnceGame
	if s.cloneBased(sl) {
		return
	}
	if int(s.g.nfx) >= maxFx {
		s.stall(StallTables)
		return
	}
	s.g.fx[s.g.nfx] = fxRec{sl: sl, kind: fxOnceStash}
	s.g.nfx++
	s.fxM.set(sl)
}

// onceBasisEnd is the settle of an ended until-end-of-turn copy: with a
// copy layer left (a live Aegis copy or a permanent marker) the printed
// mark stays aside; with none the printed abilities and their mark come
// back.
func (s *State) onceBasisEnd(sl Slot, remains bool) {
	p := &s.perms[sl]
	p.flags &^= pfOnceGame
	if remains {
		return
	}
	stashed := false
	for i := 0; s.fxM.has(sl) && i < int(s.g.nfx); i++ {
		stashed = stashed || s.g.fx[i].sl == sl && s.g.fx[i].kind == fxOnceStash
	}
	if stashed {
		s.dropFx(func(i int) bool { r := &s.g.fx[i]; return r.sl == sl && r.kind == fxOnceStash })
		p.flags |= pfOnceGame
	}
}
