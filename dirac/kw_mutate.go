package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mutate (CR 702.140, 730), cold: the pile is reached only when the table
// prints a Mutate card (genPrep.anyMutate) or a program carries
// ExtMergePermanent (genPrep.anyMerge). gorge folds a merged permanent in
// events.foldMutate (engine/events/apply_copy.go) and splits it in the Move
// fold (apply_zone.go), whose component moves run before the top's own
// placement.
//
// dirac keeps the pile as an ordered list of parked objects, one cold record
// per merged permanent. The top component's card stays in the survivor's
// perm.card, so every characteristic read sees the top face (CR 730.2a); the
// components under it park in kop.ZoneNone. An entry stores the object only:
// an object's CardID already names its current face (turnFace,
// face_multiface.go), so the card is not duplicated here.
//
// The mutating cast swaps, rather than mints. Gorge mints a parked object for
// the demoted card; dirac moves the demoted card onto the mutating spell's
// object, which is already leaving the stack, and the survivor takes the
// spell's card. The parked cards are the same set, and the kernel mints no
// non-token object (newToken sets ofToken).
//
// The cast half below (genPrep.anyMutate): gorge's mutated cast mode, the
// CR 702.140b placement ask and the resolution diversion (trig_mutates.go
// fires the trigger after the fold lands).

// mergeCap is the components under the top; the corpus's maximum is two.
const mergeCap = 4

// maxMerged is the cold region's records: the merged permanents at once, like
// maxMeldLinks.
const maxMerged = 16

// StallMerge is the merge pile's capacity stall. 46 is a free reason value
// between StallRefLatch (45) and StallZone (47).
const StallMerge = 46

// mergeRec is one merged permanent: the top object + 1 (0 free), the number
// of parked components, CR 702.140f TimesMutated, and the components in pile
// order (the first under the top first).
type mergeRec struct {
	top   objWord
	n, tm uint8
	und   [mergeCap]objWord
}

// coldMerge holds the pile records, for a table with a Mutate card or a merge
// op.
var coldMerge = newCold[mergeRec]("merge", func(g *genPrep) int {
	if g.anyMerge || g.anyMutate {
		return maxMerged
	}
	return 0
})

// DebugMergeKeep skips the split when a merged permanent leaves the
// battlefield: its components stay parked (mutation testing only: the lockstep
// must catch it).
var DebugMergeKeep bool

// prepMerge sets anyMerge (some program is ExtMergePermanent) and anyMutate
// (some card prints Mutate). It also refuses the pair kgen refuses by name
// (docs/design/mutate.md: bestow and mutate collide on the permanent spell's
// mode bits): a table printing both on one card is a build error, never a
// game.
func prepMerge(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		cd := &t.Cards[i]
		if cd.HasMutate {
			if cd.HasBestow {
				panic("kernel: card prints both Mutate and Bestow")
			}
			g.anyMutate = true
		}
		for j := range cd.Abilities {
			for _, in := range cd.Abilities[j].Prog {
				g.anyMerge = g.anyMerge || in.Ext() == kop.ExtMergePermanent
			}
		}
	}
}

func init() { stallNames[StallMerge] = "merge" }

// mergeFree is a free pile record's index, -1 when all are taken.
func (s *State) mergeFree() int {
	for i, r := range coldMerge.of(s) {
		if r.top == 0 {
			return i
		}
	}
	return -1
}

// mergeRecOf is the pile record of the merged permanent o, nil when o is
// not merged.
func (s *State) mergeRecOf(o Obj) *mergeRec {
	recs := coldMerge.of(s)
	for i := range recs {
		if recs[i].top == objWord(o)+1 {
			return &recs[i]
		}
	}
	return nil
}

// timesMutated is Count$TimesMutated's kernel read (kop.ExTimesMutated):
// the record's tm, 0 for an unmerged permanent (gorge's Object.TimesMutated).
func (s *State) timesMutated(o Obj) int {
	if rec := s.mergeRecOf(o); rec != nil {
		return int(rec.tm)
	}
	return 0
}

// mergeRoom reports whether surv has room for one more component. The
// mutating cast's offer gate reads it (kw_mutate.go below).
func (s *State) mergeRoom(surv Obj) bool {
	for _, r := range coldMerge.of(s) {
		if r.top == objWord(surv)+1 {
			return r.n < mergeCap
		}
	}
	return s.mergeFree() >= 0
}

// mergeExt is the //kmanifest:ext ExtMergePermanent handler: in.Obj is the
// survivor, s.res.e.obj the mutating spell's object, in.C != 0 the top choice.
//
//kmanifest:ext ExtMergePermanent
func (s *State) mergeExt(k *prep, in *kop.Instr) (ok, done bool) {
	if surv := s.objRefObj(in.Obj); surv != NoObj {
		s.mergeFold(k, surv, s.res.e.obj, in.C != 0, 1)
	}
	return true, true
}

// mergeFold folds src into surv's pile: top puts the spell's card on the
// survivor and parks src holding the demoted card; under parks src with the
// spell's card. Both add one component and bump TimesMutated. False (with
// StallMerge) when the record is full or none is free.
//
//go:noinline
func (s *State) mergeFold(k *prep, surv, src Obj, top bool, amount uint8) bool {
	if surv == NoObj || src == NoObj || s.objs[surv].zone != kop.ZoneBattlefield {
		return false
	}
	recs := coldMerge.of(s)
	ri := -1
	for i := range recs {
		if recs[i].top == objWord(surv)+1 {
			ri = i
			break
		}
	}
	if ri < 0 {
		ri = s.mergeFree()
		if ri < 0 {
			s.capFull(StallMerge)
			return false
		}
		recs[ri] = mergeRec{top: objWord(surv) + 1}
	}
	r := &recs[ri]
	if r.n >= mergeCap {
		s.capFull(StallMerge)
		return false
	}
	if top {
		sob := &s.objs[surv]
		old := sob.card
		sl := sob.slot
		p := &s.perms[sl]
		got := s.objs[src].card
		s.touchSl(sl)
		if p.ovr.flags&ovCopy != 0 {
			// transformObj's copy arm: the copy's Face() is its template, so
			// only the underlying card moves.
			sob.card = got
		} else {
			s.subscribe(sl, &k.cards[p.card], false)
			sob.card, p.card = got, got
			s.subscribe(sl, &k.cards[got], true)
			s.subscribeGrants(k, sl)
			if k.gen != nil {
				s.refreshTy(k)
			}
		}
		s.touch(src)
		sb := &s.objs[src]
		sb.card = old
		sb.zone = kop.ZoneNone
		sb.slot = NoSlot
		sb.ver++
		copy(r.und[1:r.n+1], r.und[:r.n])
		r.und[0] = objWord(src) + 1
		s.retargetMergeCmd(surv, src)
	} else {
		s.touch(src)
		sb := &s.objs[src]
		sb.zone = kop.ZoneNone
		sb.slot = NoSlot
		sb.ver++
		r.und[r.n] = objWord(src) + 1
	}
	r.n++
	r.tm += amount
	return true
}

// retargetMergeCmd moves each seat's commander designation with its card
// across a top merge (gorge retargets Players[p].Commanders): the demoted card
// leaves surv for src, and any card src carried moves to surv.
func (s *State) retargetMergeCmd(surv, src Obj) {
	c := s.partnerCold()
	for p := range s.pl {
		if s.pl[p].cmd == surv {
			s.pl[p].cmd = src
		} else if s.pl[p].cmd == src {
			s.pl[p].cmd = surv
		}
		if c != nil {
			if c.cmd[p] == surv {
				c.cmd[p] = src
			} else if c.cmd[p] == src {
				c.cmd[p] = surv
			}
		}
	}
}

// mergeLeaveOnLeave is the removePerm hook: a merged permanent's components
// leave with it, in pile order, before the top's own placement (zones.go).
// The gate covers the Ext op's tables and the cast's Mutate-printing tables
// alike.
func (s *State) mergeLeaveOnLeave(k *prep, o Obj, z kop.Zone, bottom bool, wasCreature bool) {
	if k.gen != nil && (k.gen.anyMerge || k.gen.anyMutate) {
		s.mergeLeave(k, o, z, bottom, wasCreature)
	}
}

// mergeLeave splits the pile topped by o as o leaves for z. Each component
// moves ZoneNone -> z through meldPlace (no replacement or trigger arm, as
// gorge's Move fold emits none for them). A graveyard move under an any-zone
// exile replacement stalls rather than split wrongly, as meldLeave does.
//
//go:noinline
func (s *State) mergeLeave(k *prep, o Obj, z kop.Zone, bottom bool, wasCreature bool) {
	recs := coldMerge.of(s)
	for i := range recs {
		if recs[i].top != objWord(o)+1 {
			continue
		}
		r := recs[i]
		recs[i] = mergeRec{}
		if DebugMergeKeep {
			return
		}
		if z == kop.ZoneGraveyard && k.gen.exileAnyC != nil {
			s.stall(StallMerge)
			return
		}
		for j := uint8(0); j < r.n; j++ {
			p := Obj(r.und[j] - 1)
			if s.objs[p].zone != kop.ZoneNone {
				continue
			}
			s.meldPlace(k, p, z, bottom, wasCreature, false)
		}
		return
	}
}

// modeMutated: stackEntry.mode on a mutating permanent spell.
const modeMutated = 1 << 0

// modeMutateTop: the placement answer, top over under (gorge
// FlagMutatedTop). Unset means under (CR 702.140b).
const modeMutateTop = 1 << 1

// mutatedEntry reports whether e is a mutating permanent spell.
func (e *stackEntry) mutatedEntry() bool {
	return e.ab == 255 && e.kind == seSpell && e.mode&(seFaceDown|modeMutated) == modeMutated
}

// offerMutateHand offers hand card o's mutating cast (gorge legal_walk_alt's
// mutate half: offerCastable at the mutate cost beside castTargetsAvailable
// over the synthesized target SA, with the pile capacity gate of
// docs/design/mutate.md: a merge that would overflow makes the cast an
// illegal option). The caller has checked the card's timing, its
// prohibition and the plain cast's gates.
//
//go:noinline
func (s *State) offerMutateHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if DebugNoMutate {
		return
	}
	cd := &k.t.Cards[c]
	bc := s.adjustCost(k, p, c, o, cd.MutateCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if !av.affords(&bc, pipMaskOf(&bc), 0) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	var buf [MaxOptions]TCode
	n := s.targetCands(k, &cd.MutateTgt, p, NoSlot, o, &buf)
	if n == 0 {
		return // gorge castTargetsAvailable over mutateTargetSA
	}
	// CR 702.140a's target stays legal through the merge; a pile at capacity
	// is not a target this kernel may take (declared-limit offer gate)
	ok := false
	for i := 0; i < n; i++ {
		if buf[i] != 0 && s.mergeRoom(Obj(buf[i])) {
			ok = true
			break
		}
	}
	if !ok {
		return
	}
	d.add(MakeAction(AkMutate, TCode(o), 0, 0))
}

// offerMutateIfPrinted is offerMutateHand for a card printed with Mutate.
func (s *State) offerMutateIfPrinted(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if k.gen != nil && k.gen.anyMutate && k.t.Cards[c].HasMutate {
		s.offerMutateHand(k, d, p, o, c, av)
	}
}

// beginMutate casts hand card o mutated. Like beginBestow: a cast the pool
// pays is an ordinary option; otherwise the planner prices it and the
// witness's sources are tapped first.
//
//go:noinline
func (s *State) beginMutate(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.mode |= modeMutated
	cost := s.buildCost(k)
	if s.sneakPoolPays(k, s.prio, s.bld().e.card, o, &cost) {
		return
	}
	s.bld().bfl |= bflLowered
	s.lowerWitness(k, s.prio)
}

// mutatePlaceOptions poses the CR 702.140b placement choice (gorge
// mutatePlaceAsk, asked once during the cast): On top / Under, gorge's
// Amount 1 and Amount 0 carried as the AkChoose mode so the answer is a
// replayable action.
func (s *State) mutatePlaceOptions(k *prep, d *Decision) {
	d.Kind, d.Player = DkChoose, s.bld().e.ctrl
	d.add(MakeAction(AkChoose, 0, 0, 1)) // On top
	d.add(MakeAction(AkChoose, 0, 0, 0)) // Under
}

// answerMutatePlace folds the placement answer onto the build's stack entry
// (gorge castAnswer's pendingCast.mutateTop, riding the pay-time CastFlags):
// the mode bit is the replayable record of the choice.
func (s *State) answerMutatePlace(k *prep, a Action) {
	if a.Kind() != AkChoose {
		return
	}
	if a.Mode()&1 != 0 {
		s.bld().e.mode |= modeMutateTop
	}
	s.nextStage(k)
	s.syncBuild()
}

// mutateResolve begins a mutating spell's resolution (CR 702.140b, 608.2b;
// gorge mutateVictim's recheck at stack.go's resolveTop head, the bestow
// arm's mirror): an illegal target ceases to be mutating — the mode bits
// clear (gorge's AlterAttribute AttrMutating removal of
// FlagMutated/FlagMutatedTop), the target is dropped and the spell falls
// through to the ordinary permanent tail, entering as its own creature
// under its controller.
//
//go:noinline
func (s *State) mutateResolve(k *prep, e *stackEntry) {
	if k.gen == nil || !k.gen.anyMutate || e.mode&modeMutated == 0 || !e.mutatedEntry() {
		return // not a mutating spell
	}
	if s.targetStillLegal(k, e, 0) {
		return
	}
	e.mode &^= modeMutated | modeMutateTop
	e.nt = 0
	e.tgt[0] = TCode(NoTarget)
	s.res.e = *e
	s.stack[s.sp-1].mode, s.stack[s.sp-1].nt, s.stack[s.sp-1].tgt[0] = e.mode, 0, TCode(NoTarget)
}

// divertMutated diverts a mutated permanent spell's resolution (resolveTop's
// permanent arm): it never becomes an independent permanent but merges into
// its target (gorge resolveMutate, before the ordinary moveResolvedOffStack).
// No battlefield slot, no enter triggers, no enter replacements — the pile
// takes no new slot, so the battlefield-full capacity draw does not apply.
// It reports whether it resolved e.
//
//go:noinline
func (s *State) divertMutated(k *prep, e *stackEntry) bool {
	if k.gen == nil || !k.gen.anyMutate || !e.mutatedEntry() {
		return false
	}
	s.sp--
	s.touch(e.obj)
	s.objs[e.obj].zone = kop.ZoneNone
	s.resolveMutate(k, e)
	s.res.active = false
	s.afterResolve()
	return true
}

// resolveMutate is a mutated spell's resolution diversion (gorge
// resolveMutate, before the ordinary moveResolvedOffStack): the pile folds
// onto the target (mergeFold above) and the spell's object parks with the
// fold — the mutating card never becomes an independent permanent
// (CR 702.140c) and fires no entry trigger; TrigMutates fires instead,
// after the fold lands so the pile's top and components are current
// (gorge folds the Mutate event before triggers run).
//
//go:noinline
func (s *State) resolveMutate(k *prep, e *stackEntry) {
	v := Obj(e.tgt[0])
	if v == NoObj || s.objs[v].zone != kop.ZoneBattlefield {
		return // the recheck holds; fail closed to no fold (unreachable in play)
	}
	s.mergeFold(k, v, e.obj, e.mode&modeMutateTop != 0, 1)
	s.fireMutates(k, s.objs[v].slot, v, e.ctrl)
}

// DebugNoMutate withholds the mutating cast (the mutate mutation test proves
// the pool casts it).
var DebugNoMutate bool
