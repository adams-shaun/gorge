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
// A bearer that is already a copy by other means (a Clone) stalls
// (StallAegis): the original copy is not saved.

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
			d.add(MakeAction(AkChoose, uint8(l.obj), 0, 0))
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
			if s.aegisLive(host) == 0 && s.perms[host].ovr.flags&ovCopy != 0 {
				s.capFull(StallAegis) // the bearer is a copy already; it is not saved
				break
			}
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

// aegisRefresh sets host's copy to that of its live attachment with the
// latest layer timestamp (skip: an attachment that is leaving), or ends it.
func (s *State) aegisRefresh(k *prep, host, skip Slot) {
	best, bestTs := -1, uint16(0)
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkAeg>>uint(i)&1 == 0 {
			continue
		}
		l := &s.g.links[i]
		ob := &s.objs[l.src]
		if ob.zone != kop.ZoneBattlefield || ob.ver != l.sver || ob.slot == skip || s.perms[ob.slot].attached != host {
			continue
		}
		if ts := s.lstamp[ob.slot]; best < 0 || ts >= bestTs {
			best, bestTs = i, ts
		}
	}
	p := &s.perms[host]
	cur := s.cardAt(host)
	var want CardID
	if best >= 0 {
		want = s.objs[s.g.links[best].obj].card
	} else {
		want = p.card
	}
	if cur == want {
		return
	}
	s.touchSl(host)
	s.subscribe(host, &k.cards[cur], false)
	if best >= 0 {
		p.ovr.flags |= ovCopy
		p.ovr.copyOf = want
	} else {
		p.ovr.flags &^= ovCopy
		p.ovr.copyOf = 0
	}
	s.subscribe(host, &k.cards[want], true)
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
