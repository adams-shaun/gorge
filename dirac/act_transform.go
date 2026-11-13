package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpTransform, cold, out of line: gorge effSetState Mode$
// Transform flips the Defined$ object to face (FaceIdx+1) % faces
// (rules effects/resolve.go) -- FlipFace, the same object, so counters,
// damage, attachments, tapped state and summoning sickness stay (CR
// 701.27c), and the object's card is its new face (dirac/face_multiface.go:
// an object's CardID is its current face's def). Gorge restamps the layer
// timestamp only for a transforming DFC (foldFlipFace IsTransformingDFC,
// CR 613.7g); a modal layout is not restamped. The kernel's FaceTransform
// layout is the transforming-DFC token.
//
// CR 701.27a: only a permanent can be transformed, so a source that left
// the battlefield in response to the ability is left untouched (gorge
// effSetState's Transform arm has the same zone gate; Forge
// SetStateEffect and XMage TransformTargetEffect/TransformSourceEffect skip
// a non-battlefield object too). FlipFace is the only transform path: the
// entering-transformed ChangeZone route is applyTransformed, not this op.
//
// The permanent's event subscriptions, static and grant masks follow the
// face: the old face's are dropped and the new one's taken, as for a copy
// (becomeCopyEnter) but in place. A permanent that is a copy is still
// represented by its own card: a DFC that copied something can transform
// (CR 712.9's Cytoshape example), advancing the hidden face under the
// copied face; a non-DFC Clone cannot (CR 712.9's Clone example). A token
// copy of a DFC is itself a double-faced token and turns over; a token copy
// of a single-faced card has no other face.

// execSetState runs OpTransform (execGen's default branch: the op is a
// reserved low number, below the generated chain's floor).
//
//go:noinline
func (s *State) execSetState(k *prep, in *kop.Instr) (ok, done bool) {
	s.transform(k, in)
	return true, true
}

// transform turns in.Obj to its next face; an object that is gone, off the
// battlefield (CR 701.27a: only a permanent transforms), or whose card has
// no other face, is left as it is.
func (s *State) transform(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	if e := &s.res.e; in.Obj == kop.RefSelf && e.kind != seSpell && s.objs[o].ver != e.srcVer {
		return // the ability's source left and returned: a new object (CR 400.7, 603.7c)
	}
	if k.anyDayNight && s.dnBound(k, o) {
		return // CR 702.145b/e: only its own daybound/nightbound turns it
	}
	s.transformObj(k, o)
}

// transformObj turns battlefield object o to its next face (transform's
// body; the day/night turns call it directly).
func (s *State) transformObj(k *prep, o Obj) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	cd := &k.t.Cards[ob.card]
	if len(cd.Faces) < 2 {
		return
	}
	if DebugIncubateNoTransform && cd.FaceMode == kop.FaceTransform {
		return
	}
	next := (int(cd.Face) + 1) % len(cd.Faces)
	sl := ob.slot
	p := &s.perms[sl]
	s.touchSl(sl)
	if p.ovr.flags&ovCopy != 0 {
		// The copy's Face() is its template (CR 712.9 example: a DFC that
		// copied a creature still shows the copied face), so only the
		// underlying FaceIdx moves, as in gorge foldFlipFace.
		ob.card = cd.Faces[next]
		if cd.FaceMode == kop.FaceTransform {
			s.ts++ // CR 613.7g: a transforming DFC gets a new layer timestamp
			s.lstamp[sl] = s.ts
		}
		s.fireTransformed(k, sl, o, p.ctrl)
		return
	}
	s.subscribe(sl, &k.cards[p.card], false)
	c := cd.Faces[next]
	ob.card, p.card = c, c
	s.subscribe(sl, &k.cards[c], true)
	s.subscribeGrants(k, sl)
	if cd.FaceMode == kop.FaceTransform {
		s.ts++ // CR 613.7g: a transforming DFC gets a new layer timestamp
		s.lstamp[sl] = s.ts
	}
	if k.gen != nil {
		s.refreshTy(k)
	}
	s.fireTransformed(k, sl, o, p.ctrl)
}
