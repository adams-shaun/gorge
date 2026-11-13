package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Assimilation Aegis: "When this enters, exile up to one target
// creature until it leaves the battlefield. Whenever this becomes attached
// to a creature, for as long as it remains attached to it, that creature
// becomes a copy of a creature card exiled with it."
//
// Three pieces, all behind genPrep.anyAegis / anyAttached:
//
//   - TrigAttached: attachTo fires the attachment's own TrigAttached
//     abilities when it moves to a different bearer (gorge attachedMatches:
//     an Attach event with a bearer; a same-bearer attach emits nothing),
//     RefTriggerObj the bearer.
//   - ExtAegisClone, the trigger's body: the controller picks one of the
//     creature cards exiled with this incarnation (the OpExileUntilLeaves
//     link, ver-checked as CR 400.7 wants; gorge son-exiledwith-cr4007) and
//     the bearer becomes a copy of it. The ask is gorge's clone_choice
//     KChoose (posed whatever the pool size); it rides the Hideaway ask
//     code (raHideaway) with the instruction telling the two apart and
//     answers as ChGenChooseCard ("card").
//   - UntilUnattached: genState.linkAeg bit i marks link i as an attachment
//     whose copy is live. The copy ends the moment the attachment is
//     unattached or leaves (gorge son-clone-untilunattached): attachTo to
//     another bearer, the attachment leaving (removePerm), its bearer
//     leaving (removePerm), the Equipment SBA. The bearer's copy is the
//     live attachment with the latest layer timestamp (CR 613.7e;
//     lstamp), else it is its own card again.
//
// A bearer that is already a copy by other means (a Clone, Fleeting
// Reflection) keeps every layer: the Aegis copy is the newest, so it wins
// (CR 613.7e), and the newest older copy shows again once the Aegis copy
// ends (copy_aegisunder.go copyTop). A copy with no marker of its own (a
// token minted as a copy) waits in fxAegisBase and shows again with the last
// Aegis copy's end.

// DebugAegisNoUnlink (test only) keeps a copy after its attachment is unattached.
var DebugAegisNoUnlink bool

// aegisPool is the creature cards exiled with the attachment's incarnation
// (o, ver): their count and the first link index.
func (s *State) aegisPool(k *prep, o Obj, ver uint8) (n, first int) {
	first = -1
	for i := 0; i < int(s.g.nlinks); i++ {
		l := &s.g.links[i]
		if l.src != o || l.sver != ver || s.g.linkHide>>uint(i)&1 != 0 || s.g.linkHand>>uint(i)&1 != 0 {
			continue
		}
		x := &s.objs[l.obj]
		if x.zone != kop.ZoneExile || k.cards[x.card].types&kop.TCreature == 0 {
			continue
		}
		if first < 0 {
			first = i
		}
		n++
	}
	return n, first
}

// fireAttached queues the TrigAttached abilities of the attachment at sl,
// which just became attached to host.
//
//go:noinline
func (s *State) fireAttached(k *prep, sl, host Slot) {
	if s.typesAt(k, host)&kop.TCreature == 0 {
		return // ValidTarget$ Creature
	}
	o := s.perms[sl].obj
	ob := &s.objs[o]
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbTriggered || a.Trig.Event != kop.TrigAttached {
			continue
		}
		c := s.cardAt(sl)
		np := s.npend
		s.enqueueLKI(o, ob.ver, c, uint8(i), s.perms[sl].ctrl, s.perms[host].obj, s.perms[host].ctrl, 0)
		s.panharmKeyword(k, sl, kop.TrigAttached, s.perms[host].obj, np)
	}
}

// aegisStart is ExtAegisClone: poses the pick.
//
//go:noinline
func (s *State) aegisStart(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return true // the attachment left: its exile pool went back
	}
	n, _ := s.aegisPool(k, e.obj, e.srcVer)
	if n == 0 {
		return true // gorge: no eligible object, no copy
	}
	if n > 1 {
		s.capFull(StallAegis) // an Aegis exiles at most one card per incarnation
		return true
	}
	s.res.ask, s.res.askP = raHideaway, e.ctrl
	return false
}

// aegisOptions lists the pick (askOptionsHide's other half).
func (s *State) aegisOptions(k *prep, d *Decision) {
	e := &s.res.e
	d.Sub = ChGenChooseCard
	for i := 0; i < int(s.g.nlinks); i++ {
		l := &s.g.links[i]
		if l.src != e.obj || l.sver != e.srcVer || s.g.linkHide>>uint(i)&1 != 0 || s.g.linkHand>>uint(i)&1 != 0 {
			continue
		}
		x := &s.objs[l.obj]
		if x.zone == kop.ZoneExile && k.cards[x.card].types&kop.TCreature != 0 {
			d.add(MakeAction(AkChoose, TCode(l.obj), 0, 0))
		}
	}
}

// aegisApply takes the pick: the bearer becomes a copy of the card.
func (s *State) aegisApply(k *prep, a Action) {
	e := &s.res.e
	s.res.ask = raNone
	host := s.objRef(k, kop.RefTriggerObj)
	ob := &s.objs[e.obj]
	if host != NoSlot && ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer && s.perms[ob.slot].attached == host {
		for i := 0; i < int(s.g.nlinks); i++ {
			l := &s.g.links[i]
			if l.src != e.obj || l.sver != e.srcVer || l.obj != Obj(a.Source()) {
				continue
			}
			// The first Aegis copy over a copy no marker records (a token
			// minted as a copy, or an untracked entering copy) keeps that
			// copy aside for the last Aegis copy's end.
			if s.aegisLive(host) == 0 && s.perms[host].ovr.flags&ovCopy != 0 &&
				s.copyRec(host, fxCopyPerm) < 0 && s.copyRec(host, fxCopyEOT) < 0 && !s.aegisUnderPush(host) {
				break // the effect pool is full
			}
			l.ts = s.tick()
			s.g.linkAeg |= 1 << uint(i)
			s.aegisRefresh(k, host, NoSlot)
			break
		}
	}
	s.continueRes(k, 0)
}

// aegisLive is the number of live Aegis copies on host.
func (s *State) aegisLive(host Slot) int {
	n := 0
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkAeg>>uint(i)&1 == 0 {
			continue
		}
		l := &s.g.links[i]
		if ob := &s.objs[l.src]; ob.zone == kop.ZoneBattlefield && ob.ver == l.sver && s.perms[ob.slot].attached == host {
			n++
		}
	}
	return n
}

// aegisRefresh leaves host on its newest live copy layer (copyTop): a live
// Aegis copy, a newer clone, or the layer the Aegis stack covered. skip is
// an attachment that is leaving.
func (s *State) aegisRefresh(k *prep, host, skip Slot) {
	l := s.copyTop(host, skip)
	if l.kind == copyNone {
		// the floor beneath every tracked layer: the copy the Aegis stack
		// covered that has no marker of its own
		if c, ok := s.aegisUnderTake(host); ok {
			l = copyLayer{kind: copyBase, card: c}
		}
	}
	p := &s.perms[host]
	cur := s.cardAt(host)
	want := p.card
	if l.kind != copyNone {
		want = l.card
	}
	if cur == want && (l.kind != copyNone) == (p.ovr.flags&ovCopy != 0) && s.excShows(host, l) {
		return
	}
	s.touchSl(host)
	s.subscribe(host, &k.cards[cur], false)
	oldKw := s.copyExcKw(k, host)
	if l.kind != copyNone {
		p.ovr.flags |= ovCopy
		p.ovr.copyOf = want
		switch l.kind {
		case copyPerm, copyEOT:
			// The layer's copy exceptions come back with it: its template's
			// (inherited, CR 707.3) and then its own override (CR 707.9b).
			// The layers beneath lose theirs; they come back with their own
			// layer.
			s.dropFx(func(i int) bool {
				x := &s.g.fx[i]
				return x.sl == host && x.kind == fxCopyExc
			})
			s.copyExcRebase(k, host, l) // and its AddAbilities$ grants (copyGrantsRebase)
			p.ovr.kw = p.ovr.kw&^oldKw | s.copyExcKw(k, host)
		case copyAegis:
			// An Aegis copy has no exceptions, so an older "except it has
			// hexproof" is overwritten while it shows (CR 707.2, 613.7e).
			// A copy the stack covered that has no marker keeps its
			// exceptions aside: only its own layer can give them back.
			// With a marker layer live beneath, or once the base's records
			// were already dealt with (fl set), the live records are a
			// marker layer's, which re-pushes its own.
			bi := s.copyRec(host, fxAegisBase)
			if bi >= 0 && s.g.fx[bi].r.fl == 0 && s.copyRec(host, fxCopyPerm) < 0 && s.copyRec(host, fxCopyEOT) < 0 {
				s.g.fx[bi].r.fl = 1
				s.aegisHold(host)
			} else {
				if bi >= 0 {
					s.g.fx[bi].r.fl = 1
				}
				s.dropFx(func(i int) bool {
					x := &s.g.fx[i]
					return x.sl == host && x.kind == fxCopyExc
				})
				s.dropCopyGrants(host)
			}
			p.ovr.kw = p.ovr.kw&^oldKw | s.copyExcKw(k, host)
		case copyBase:
			s.aegisRestore(host)
			p.ovr.kw = p.ovr.kw&^oldKw | s.copyExcKw(k, host)
		}
	} else {
		p.ovr.flags &^= ovCopy
		p.ovr.copyOf = 0
		s.dropCopyGrants(host)
		s.cloneRestore(host) // a token copy's own basis
	}
	s.subscribe(host, &k.cards[s.cardAt(host)], true)
	s.refreshTy(k)
}

// aegisUnlink ends the live copy of the attachment at sl, if it has one,
// and refreshes the bearer (host) it leaves; keep: the attachment stays on
// the battlefield, otherwise it is the one leaving (skipped).
//
//go:noinline
func (s *State) aegisUnlink(k *prep, sl, host Slot, leaving bool) {
	if s.g.linkAeg == 0 || DebugAegisNoUnlink {
		return
	}
	o := s.perms[sl].obj
	ver := s.objs[o].ver
	for i := 0; i < int(s.g.nlinks); i++ {
		l := &s.g.links[i]
		if s.g.linkAeg>>uint(i)&1 == 0 || l.src != o || l.sver != ver {
			continue
		}
		s.g.linkAeg &^= 1 << uint(i)
		if host != NoSlot && s.occ.has(host) {
			skip := NoSlot
			if leaving {
				skip = sl
			}
			s.aegisRefresh(k, host, skip)
		}
	}
}

// aegisMoved runs after the attachment at sl moved from old (NoSlot: it was
// unattached) to host: its copy of the old bearer ends, then it "becomes
// attached".
//
//go:noinline
func (s *State) aegisMoved(k *prep, sl, old, host Slot) {
	if old != NoSlot {
		s.aegisUnlink(k, sl, old, false)
	}
	s.fireAttached(k, sl, host)
}

// aegisLeaves runs from leaveGen, as the object o left slot sl (its perm
// record is still intact): a copy it is wearing is unsubscribed, and every
// live copy whose attachment is gone (o itself, or its bearer left so the
// attachment is unattached) ends; a bearer that outlives its attachment is
// refreshed.
//
//go:noinline
func (s *State) aegisLeaves(k *prep, sl Slot, o Obj) {
	if s.g.linkAeg == 0 {
		return
	}
	if p := &s.perms[sl]; p.ovr.flags&ovCopy != 0 {
		s.subscribe(sl, &k.cards[p.ovr.copyOf], false)
	}
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkAeg>>uint(i)&1 == 0 {
			continue
		}
		l := &s.g.links[i]
		ob := &s.objs[l.src]
		switch {
		case l.src == o:
			s.g.linkAeg &^= 1 << uint(i)
			if h := s.perms[sl].attached; h != NoSlot && s.occ.has(h) && !DebugAegisNoUnlink {
				s.aegisRefresh(k, h, NoSlot)
			}
		case ob.zone != kop.ZoneBattlefield || ob.ver != l.sver || s.perms[ob.slot].attached == NoSlot:
			s.g.linkAeg &^= 1 << uint(i) // its bearer left: nothing to restore
		}
	}
}

// prepAegis sets anyAegis.
func prepAegis(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigAttached {
				g.anyAegis = true
				return
			}
			for _, in := range a.Prog {
				if in.Ext() == kop.ExtAegisClone {
					g.anyAegis = true
					return
				}
			}
		}
	}
}

// excShows reports whether host's copy-exception records already are the ones
// layer l shows: an Aegis copy has none, and a copy marker shows its
// template's exceptions (CR 707.3) and then its own override (CR 707.9b). A
// self-copy's template is host itself, so its inherited records are the held
// ones (excSelfMatch).
func (s *State) excShows(host Slot, l copyLayer) bool {
	switch l.kind {
	case copyAegis:
		return s.copyRec(host, fxCopyExc) < 0 && s.copyGrantsShow(host, l)
	case copyPerm, copyEOT:
		if l.src != NoObj && l.src == s.perms[host].obj {
			return s.excSelfMatch(host, l) && s.copyGrantsShow(host, l)
		}
		return s.excMatch(host, s.copySrcSlot(l.src), l.idx) && s.copyGrantsShow(host, l)
	case copyBase:
		// a kept floor is always restored by the refresh that takes it
		return false
	}
	return true
}

// excSelfMatch reports whether slot sl's copy-exception records are exactly
// what copyExcRebase pushes for self layer l: the held inherited overrides in
// pool order, then the override idx.
func (s *State) excSelfMatch(sl Slot, l copyLayer) bool {
	at := 0
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != sl || r.kind != fxCopySelf || r.r.ts != l.ts || r.r.fl&ovCopyGrant != 0 {
			continue
		}
		if got, ok := s.excIth(sl, at); !ok || got != r.r.idx {
			return false
		}
		at++
	}
	if l.idx != 0 {
		if got, ok := s.excIth(sl, at); !ok || got != l.idx {
			return false
		}
		at++
	}
	_, more := s.excIth(sl, at)
	return !more
}

// excMatch reports whether slot sl's copy-exception records are exactly what
// copyExcAdd(sl, src, idx) would push: src's records in pool order, then the
// override idx.
func (s *State) excMatch(sl, src Slot, idx uint8) bool {
	at := 0
	if src != NoSlot && s.fxM.has(src) {
		for i := 0; i < int(s.g.nfx); i++ {
			r := &s.g.fx[i]
			if r.sl != src || r.kind != fxCopyExc {
				continue
			}
			if got, ok := s.excIth(sl, at); !ok || got != r.r.idx {
				return false
			}
			at++
		}
	}
	if idx != 0 {
		if got, ok := s.excIth(sl, at); !ok || got != idx {
			return false
		}
		at++
	}
	_, more := s.excIth(sl, at)
	return !more
}

// excIth is the k-th (0-based) copy-exception override on slot sl.
func (s *State) excIth(sl Slot, k int) (idx uint8, ok bool) {
	if !s.fxM.has(sl) {
		return 0, false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			if k == 0 {
				return r.r.idx, true
			}
			k--
		}
	}
	return 0, false
}
