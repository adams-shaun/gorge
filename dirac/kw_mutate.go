package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mutate (CR 702.140, 730), cold: reached only when the table holds an
// OpExt ExtMergePermanent (genPrep.anyMerge). gorge folds a merged permanent
// in events.foldMutate (engine/events/apply_copy.go) and splits it in the
// Move fold (apply_zone.go), whose component moves run before the top's own
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

// coldMerge holds the pile records, for a table with a merge op.
var coldMerge = newCold[mergeRec]("merge", func(g *genPrep) int {
	if g.anyMerge {
		return maxMerged
	}
	return 0
})

// DebugMergeKeep skips the split when a merged permanent leaves the
// battlefield: its components stay parked (mutation testing only: the lockstep
// must catch it).
var DebugMergeKeep bool

// prepMerge sets anyMerge: some program is ExtMergePermanent.
func prepMerge(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
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

// mergeRoom reports whether surv has room for one more component. Ticket 2's
// legality check reads it.
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
func (s *State) mergeLeaveOnLeave(k *prep, o Obj, z kop.Zone, bottom bool, wasCreature bool) {
	if k.gen != nil && k.gen.anyMerge {
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
