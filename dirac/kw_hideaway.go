package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Hideaway (Collector's Cage; CR 702.75a; gorge effHideaway, effPlay).
//
// ExtHideaway C 0, the ETB: the controller looks at the top B cards of their
// library (all of it when shorter), is asked which one to exile face down
// (ChHideaway, gorge's KChoose "hideaway_pick", the options in library
// order), and the rest go to the bottom in a random order: gorge
// moveRestToBottom's backward Fisher-Yates (Rand(i+1) for i from the last
// index down to 1; none for a single card). The exiled card is linked to
// the source's incarnation in genState.links (src, ver of the ETB trigger's
// source, the card) with the genState.linkHide bit set, so leaveGen's
// "exiled until the source leaves" return never touches it.
//
// ExtHideaway C 1, the Play: the card linked to this ability's source
// incarnation (gorge Defined$ ExiledWith, scanning exile) is offered as an
// OpCastFree pick (ChGenCastFree, Optional$, WithoutManaCost$). A land is
// played at once instead of queued (gorge beginPlay: effectLandPlayRefusal,
// the active player with a land play left, then the land play counts).
//
// The link ends when the card leaves exile (detach) and when its source
// incarnation is gone with nothing of it left on the stack (hideGC, run
// only when the link table is full). gorge keys ExiledWith on the object id
// alone, so a source that left and returned still sees the card; the
// kernel follows CR 400.7 (a new object has exiled nothing). All behind
// genPrep.anyHideaway.

const raHideaway uint8 = 230

// ChHideaway is gorge's "hideaway_pick" KChoose: source = a looked-at card.
const ChHideaway ChoiceKind = 230

// prepHideaway sets anyHideaway.
func prepHideaway(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtHideaway {
					g.anyHideaway = true
					return
				}
			}
		}
	}
}

// hideStart is ExtHideaway C 0.
//
//go:noinline
func (s *State) hideStart(k *prep, in *kop.Instr) bool {
	p := s.res.e.ctrl
	pl := &s.pl[p]
	n := int(in.B.N)
	if l := pl.libLen(); n > l {
		n = l
	}
	if n <= 0 {
		return true
	}
	if n > len(s.resArm().look) {
		s.capFull(StallLook)
		n = len(s.resArm().look)
	}
	s.g.pickP = p
	s.g.nlook = uint8(n)
	for i := 0; i < n; i++ {
		s.resArm().look[i] = pl.lib[int(pl.libLo)+i]
	}
	s.res.ask, s.res.askP = raHideaway, p
	return false
}

// askOptionsHide builds the Hideaway pick (the tail of askOptionsColGrant).
//
//go:noinline
func (s *State) askOptionsHide(k *prep, d *Decision) bool {
	if s.res.ask != raHideaway {
		return s.askOptionsNumTimes(k, d) // cast_announcetimes.go
	}
	if s.prog(k)[s.res.pc].Ext() == kop.ExtAegisClone {
		s.aegisOptions(k, d) // zone_exilethenattach.go
		return true
	}
	if s.prog(k)[s.res.pc].Ext() == kop.ExtCloneCopy {
		s.cloneCopyOptions(k, d) // copy_clone.go
		return true
	}
	d.Sub = ChHideaway
	for i := 0; i < int(s.g.nlook); i++ {
		d.add(MakeAction(AkChoose, TCode(s.resArm().look[i]), 0, 0))
	}
	return true
}

// applyAskHide exiles the pick face down, links it and bottoms the rest.
//
//go:noinline
func (s *State) applyAskHide(k *prep, a Action) bool {
	if s.res.ask != raHideaway {
		return s.applyAskNumTimes(k, a) // cast_announcetimes.go
	}
	if s.prog(k)[s.res.pc].Ext() == kop.ExtAegisClone {
		s.aegisApply(k, a) // zone_exilethenattach.go
		return true
	}
	if s.prog(k)[s.res.pc].Ext() == kop.ExtCloneCopy {
		s.cloneApply(k, a) // copy_clone.go
		return true
	}
	o := Obj(a.Source())
	pl := &s.pl[s.g.pickP]
	nr := int(s.g.nlook) - 1
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneExile, false, from, false, false)
	s.hideLink(o)
	if nr > 0 {
		var rest [24]Obj // len(genState.look)
		for i := 0; i < nr; i++ {
			rest[i] = pl.lib[int(pl.libLo)+i]
			s.touch(rest[i])
		}
		for i := nr - 1; i > 0 && !DebugHideNoShuffle; i-- {
			j := s.rng.IntN(i + 1)
			rest[i], rest[j] = rest[j], rest[i]
		}
		pl.libLo += uint8(nr)
		for i := 0; i < nr; i++ {
			pl.libBottom(rest[i])
		}
	}
	s.g.nlook = 0
	s.continueRes(k, 0)
	return true
}

// hideLink links exiled card o to the resolving entry's source incarnation.
func (s *State) hideLink(o Obj) {
	e := &s.res.e
	if s.g.nlinks >= uint8(len(s.g.links)) {
		s.hideGC()
	}
	if s.g.nlinks >= uint8(len(s.g.links)) {
		s.capFull(StallLinks)
		return
	}
	i := s.g.nlinks
	s.g.links[i].src, s.g.links[i].sver, s.g.links[i].obj = e.obj, e.srcVer, o
	s.g.linkHide |= 1 << i
	s.g.nlinks++
}

// hideDrop removes link i, shifting the masks of the links above it.
func (s *State) hideDrop(i int) {
	copy(s.g.links[i:], s.g.links[i+1:s.g.nlinks])
	s.g.nlinks--
	s.dropLinkHand(i) // act_mayask.go: both masks shift
}

// hideUnlinkObj drops the hideaway link of card o, which is leaving exile
// (detach).
//
//go:noinline
func (s *State) hideUnlinkObj(o Obj) {
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkHide>>uint(i)&1 != 0 && s.g.links[i].obj == o {
			s.hideDrop(i)
			return
		}
	}
}

// hideGC drops the links whose source incarnation is gone and has nothing
// left on the stack or waiting to be put on it.
//
//go:noinline
func (s *State) hideGC() {
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkHide>>uint(i)&1 == 0 {
			continue
		}
		l := s.g.links[i]
		if ob := &s.objs[l.src]; ob.zone == kop.ZoneBattlefield && ob.ver == l.sver {
			continue
		}
		live := false
		for j := 0; j < int(s.sp) && !live; j++ {
			e := &s.stack[j]
			live = e.kind != seSpell && e.obj == l.src && e.srcVer == l.sver
		}
		for j := 0; j < int(s.npend) && !live; j++ {
			live = s.pend[j].obj == l.src && s.pend[j].srcVer == l.sver
		}
		if s.res.active && s.res.e.kind != seSpell && s.res.e.obj == l.src && s.res.e.srcVer == l.sver {
			live = true
		}
		if !live {
			s.hideDrop(i)
			i--
		}
	}
}

// hideHas reports whether card o is a Hideaway card (face down in exile).
func (s *State) hideHas(o Obj) bool {
	for i := 0; i < int(s.g.nlinks); i++ {
		if s.g.linkHide>>uint(i)&1 != 0 && s.g.links[i].obj == o {
			return true
		}
	}
	return false
}

// hideLinked reports whether card o is linked to the resolving entry's
// source incarnation.
func (s *State) hideLinked(o Obj) bool {
	e := &s.res.e
	for i := 0; i < int(s.g.nlinks); i++ {
		if l := &s.g.links[i]; s.g.linkHide>>uint(i)&1 != 0 && l.obj == o && l.src == e.obj && l.sver == e.srcVer {
			return true
		}
	}
	return false
}

// hidePlay is ExtHideaway C 1: the linked card as a free-cast pick.
//
//go:noinline
func (s *State) hidePlay(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	s.g.nfree, s.g.nfreeAll = 0, 0
	if s.g.linkHide == 0 {
		return true
	}
	for p := 0; p < int(s.nSeats()); p++ {
		if s.out(uint8(p)) {
			continue
		}
		pl := &s.pl[p]
		for i := 0; i < int(pl.exileN); i++ {
			o := pl.exile[i]
			if !s.hideLinked(o) {
				continue
			}
			c := s.objs[o].card
			if k.cards[c].land {
				// gorge host_read.go CastProhibited: a land the play could not
				// make (not the active player, no land play left) is not offered
				if e.ctrl != s.active || s.pl[e.ctrl].landsPlayed >= s.landLimit(k, e.ctrl) {
					continue
				}
			} else if !s.spellTargetsFeasible(k, c, e.ctrl) || k.gen.anyCantCast && s.castForbiddenFree(k, e.ctrl, c, o) {
				continue
			}
			if s.g.nfree >= uint8(len(s.g.free)) {
				s.capFull(StallFree)
				break
			}
			s.g.free[s.g.nfree] = o
			s.g.nfree++
		}
	}
	copy(s.g.freeAll[:s.g.nfree], s.g.free[:s.g.nfree])
	s.g.nfreeAll = s.g.nfree
	if s.g.nfree == 0 {
		return true
	}
	s.g.freeP = seatWord(e.ctrl) | seatWord(kop.ZoneExile)<<freeZoneShift
	s.res.ask, s.res.askP = raGenCastFree, e.ctrl
	return false
}

// hideLandNow is the pick answer's land branch (interp_ask.go): the as-
// enters choice and the CR 616.1 entry-order ask (repl_entryorder.go) are
// posed before the move when gorge poses one there, and the land is played
// now, not queued. The pick offer resumes when nothing was asked.
func (s *State) hideLandNow(k *prep, o Obj) {
	if s.hideLandPark(k, o) || s.hideLand(k, o) {
		return // its as-enters choice or entry-order ask, the card still in exile
	}
	if s.g.nfree == 0 {
		s.castFreeClosed(k)
	}
}

// hideLand plays the answered land (gorge beginPlay: the active player with
// a land play left; otherwise one Note and nothing). It parks the land's CR
// 616.1 entry-order ask (repl_entryorder.go) with the card still in exile
// when a tap and an untap compete over its entry: true then, the answer
// resumes the play (hideEntryAnswer). The land play is counted only on the
// run that makes the move (gorge counts it after the order ask, at the move,
// where playLandNow's route counts before it).
//
//go:noinline
func (s *State) hideLand(k *prep, o Obj) bool {
	p := s.res.e.ctrl
	if p != s.active || s.pl[p].landsPlayed >= s.landLimit(k, p) || s.objs[o].zone != kop.ZoneExile {
		return false
	}
	if s.entryOrderPark(k, o, p, entOp) {
		s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p
		return true
	}
	from := s.detach(o)
	if k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	deferCount := k.gen.has(s.objs[o].card, gfShockEntry) // the election's answer completes the play
	if !deferCount {
		s.pl[p].landsPlayed++
	}
	if k.gen.anyLandPlayed {
		s.enterLandPlayed(k, o, p, from)
	} else {
		s.enter(k, o, p, false, from)
	}
	if deferCount {
		s.shockLandCount(k, o, p)
	}
	return false
}

// Debug switches for the cage mutation tests.
var (
	// DebugHideNoShuffle keeps the bottomed cards in library order.
	DebugHideNoShuffle bool
	// DebugHideNoLand queues a hidden land like a spell (no land play).
	DebugHideNoLand bool
)

// preHideLand is preAsk.state for a hidden land a Play answer picked, waiting
// in exile for its as-enters choice: gorge's land play asks it before the
// move (CR 614.12, the hand play's preLandAsk), and the answer plays it.
const preHideLand uint8 = 9

// hideLandPark poses the picked land's as-enters choice (Crossroads Village's
// colour) with the card still in exile; false when the land enters at once
// (no such choice, or a play hideLand refuses).
//
//go:noinline
func (s *State) hideLandPark(k *prep, o Obj) bool {
	p := s.res.e.ctrl
	if p != s.active || s.pl[p].landsPlayed >= s.landLimit(k, p) || s.objs[o].zone != kop.ZoneExile {
		return false
	}
	switch kind := s.enterChoiceKind(k, s.objs[o].card); kind {
	case akColor, akType, akNumber, akName, akNameAny:
		s.g.pre = preAsk{obj: o, kind: kind, state: preHideLand}
		s.res.ask, s.res.askP = raGenRetChoice, p
		return true
	}
	return false
}

// hideLandAnswer plays the parked land with its answered choice (the
// enters program reads pre.val), then the Play's pick goes on.
//
//go:noinline
func (s *State) hideLandAnswer(k *prep) {
	o := s.g.pre.obj
	s.g.pre.state = 2
	s.res.ask = raGenCastFree
	if s.hideLand(k, o) {
		return // the answered choice stays for the enter; the entry-order ask resumes the play (hideEntryAnswer)
	}
	s.hideLandTail(k)
}

// hideLandTail clears the answered as-enters choice and resumes the pick
// offer (every hideLand path ends here).
func (s *State) hideLandTail(k *prep) {
	s.g.pre = preAsk{}
	if s.g.nfree == 0 {
		s.castFreeClosed(k)
	}
}

// hideEntryAnswer answers a parked hidden land's entry-order ask
// (repl_entryorder.go): the play runs again, the enter consumes the order's
// result; a tap and an untap still left (CR 616.1f) re-ask before the pick
// offer goes on.
//
//go:noinline
func (s *State) hideEntryAnswer(k *prep) {
	s.res.ask = raGenCastFree
	s.hideLand(k, s.g.ro.entObj()) // the play is counted here, at the move, as gorge does after the ask
	if s.g.ro.n > 0 && s.g.ro.pcol&entLate != 0 {
		s.res.ask, s.res.askP = raGenEntryOrder, s.g.ro.p // CR 616.1f: the rest is asked once the entry is done
		return
	}
	s.hideLandTail(k)
	s.entryOrderClear()
}

// hideEntryPick takes an answer to the hidden land's order ask.
func (s *State) hideEntryPick(k *prep, a Action) {
	switch s.entryOrderPick(k, a) {
	case entAsk:
		return
	case entFin:
		// the late tail's last answer: the order is applied (the pick
		// cleared ro), the pick offer goes on
		s.res.ask = raGenCastFree
		s.hideLandTail(k)
	case entGo:
		s.hideEntryAnswer(k)
	}
}

// hideEntryParked reports the parked entry-order ask is a hidden land's
// (kw_hideaway.go): the resolution sits at the ExtHideaway play op, while
// the OpReturnEx routes the card moves park on rerun (entryRerunOp) and a
// parked copy token's ask was taken by entryCopyParked.
func (s *State) hideEntryParked(k *prep) bool {
	ro := &s.g.ro
	if !ro.entHdr() || ro.pcol&entMode != entOp {
		return false
	}
	in := &s.prog(k)[s.res.pc]
	return in.Op == kop.OpExt && in.Ext() == kop.ExtHideaway && in.C == 1
}
