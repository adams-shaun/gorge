package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Myriad (CR 702.116a; gorge effMyriad, effects/myriad.go): "whenever this
// creature attacks, for each opponent other than the defending player, you
// may create a token that's a copy of this creature that's tapped and
// attacking that player or a planeswalker they control. Exile the tokens at
// end of combat." ExtMyriad (kgen lower_myriad.go) is the trigger's whole
// body.
//
// The opponents are the living players from the controller in turn order
// (gorge AliveFrom) but the controller and the defending player, the one the
// trigger's attack event named as it was queued (stackEntry.defSeat; gorge
// carries the DeclareAttackers event's Player in the trigger's Remembered).
// The controller is asked about each in turn (raGenMyriad, ChGenMyriad: gorge
// poses a KChoose over the source), res.left holds the opponent asked about,
// and a yes makes the copy op's token copies of the source (Obj, read as
// OpCopyAsTokenEx reads Obj, B and C; CpTapped) at once, each entering
// tapped and attacking that player. A copy that parks an ask stops the game
// (StallCopyAllAsk, as the copy ops always do).
//
// CR 702.116a's walker arm ("or a planeswalker they control"; gorge offers
// the asked player's face-up planeswalkers beside the player, in battlefield
// order, the CR 508.4 arm Adeline's token ask poses): myriadOptions poses the
// same options, a walker answer rides the action's Source (a player's yes
// rides the Myriad source, as in gorge, where the player option's Obj is the
// source) and the copy enters attacking that walker (the answer rides
// g.tokDef across the copy walk, as an Attacking$ True entry's pre-asked
// defender does).
//
// The exile is the token's own delayed triggered ability (CR 702.116a /
// 511.2; gorge registers the AtEOT$ ExileCombat shape per mint,
// effects/ateot.go scheduleAtEOTRider __kwWarpExile at this turn's
// end-of-combat step): myrCopyEnter records one dlyEndCombat OpExile promise
// on each copy, fireDlyEndCombat enqueues it as the step begins, the
// resolution exiles the copy, and fireDelayed drops one still pooled at the
// turn's end step (gorge's |TT= expiry; a promise whose copy died in combat
// still fires and does nothing, CR 603.7). gorge's IsMyriad flag on the copy
// is not modelled; nothing reads it past the exile.

// DebugMyriadNoOpp is the mutation switch: no opponent is asked about, the
// old two-seat reading ("no opponent other than the defending player").
// Tests only.
var DebugMyriadNoOpp bool

// DebugMyriadNoWalker is the mutation switch: the walker arm is not offered
// (the player-and-decline pair only). Tests only.
var DebugMyriadNoWalker bool

// StallMyriadPW is no longer raised: the walker arm is modelled
// (myriadOptions). The number keeps its name.
const StallMyriadPW = 179

func init() { stallNames[StallMyriadPW] = "myriadpw" }

// myriadExt is OpExt ExtMyriad: the first opponent's ask, or nothing.
//
//kmanifest:ext ExtMyriad
func (s *State) myriadExt(k *prep, in *kop.Instr) (ok, done bool) {
	return !s.myriadAsk(k, noSeat), true
}

// myriadAsk asks about the next opponent after seat after (noSeat: the
// first); false when none is left or the game stalled.
func (s *State) myriadAsk(k *prep, after uint8) bool {
	if DebugMyriadNoOpp {
		return false
	}
	e := &s.res.e
	def := s.oneOppOf(k, kop.RefDefendingPlayer, e, e.ctrl, e.obj)
	q := s.oppsFrom(e.ctrl)
	i := uint8(0)
	if after != noSeat {
		for i < q.n && q.s[i] != after {
			i++
		}
		i++
	}
	for ; i < q.n; i++ {
		if q.s[i] != def {
			s.res.ask, s.res.askP, s.res.left = raGenMyriad, e.ctrl, q.s[i]
			return true
		}
	}
	return false
}

// myriadOptions poses gorge's options over the asked opponent res.left: yes
// (the player, over the source), yes for each face-up planeswalker they
// control (battlefield order, over the walker), then no (myriadAskOptions).
func (s *State) myriadOptions(k *prep, d *Decision) {
	d.Sub = ChGenMyriad
	src := TCode(s.res.e.obj)
	d.add(MakeAction(AkChoose, src, 0, 1))
	var buf [MaxOptions]TCode
	for i, n := 0, s.myriadWalkers(k, s.res.left, &buf); i < n; i++ {
		d.add(MakeAction(AkChoose, buf[i], 0, 1))
	}
	d.add(MakeAction(AkChoose, src, 0, 0))
}

// myriadWalkers lists seat q's face-up planeswalkers in battlefield order
// (gorge myriadAskOptions's walker candidates, CR 702.116a).
func (s *State) myriadWalkers(k *prep, q uint8, buf *[MaxOptions]TCode) int {
	if DebugMyriadNoWalker {
		return 0
	}
	var m slotMask
	for c := s.ctrlM[q].and(s.occ); c.any(); c = c.rest() {
		if sl := c.first(); !s.faceDown(k, sl) && s.typesAt(k, sl)&kop.TPlaneswalker != 0 {
			m = m.with(sl)
		}
	}
	var order [MaxPerm]Slot
	c := s.battlefieldOrder(q, m, order[:], 0)
	n := 0
	for _, sl := range order[:c] {
		if n < MaxOptions-2 {
			buf[n] = TCode(s.perms[sl].obj)
			n++
		}
	}
	return n
}

// myriadAnswer makes the copy on a yes, then asks about the next opponent.
func (s *State) myriadAnswer(k *prep, a Action) {
	r := &s.res
	opp := r.left
	r.ask = raNone
	if a.Mode() == 1 {
		if src := TCode(r.e.obj); a.Source() != src { // the walker arm: the copy attacks it
			s.g.tokDef, s.g.tokDefSet = a.Source(), true
		}
		s.myriadCopy(k, &s.prog(k)[r.pc], opp)
		s.g.tokDefSet = false
		if s.over {
			return
		}
	}
	if !s.myriadAsk(k, opp) && s.g.stalled == 0 {
		s.continueRes(k, 0)
	}
}

// myriadCopy makes the source's copies attacking opponent opp.
func (s *State) myriadCopy(k *prep, in *kop.Instr, opp uint8) {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	cin := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: in.Obj, A: kop.Const(1), B: in.B, C: in.C}
	s.res.flags &^= rfCopyGrp
	s.encoreCopy(k, &cin, c, opp) // kw_encore.go: the per-opponent copy walk, entering under rfOppCopy
	s.res.flags &^= rfCopyGrp
}

// myrCopyEnter is a Myriad copy's entry (oppCopyEnter, under rfOppCopy):
// tapped and attacking the opponent res.left, carrying its end-of-combat
// exile promise (CR 702.116a / 511.2).
func (s *State) myrCopyEnter(k *prep, in *kop.Instr, sl Slot, t Obj) {
	atk := PlayerCode(s.res.left)
	if s.g.tokDefSet { // the walker arm's answer (myriadAnswer)
		atk = s.g.tokDef
	}
	s.setEnterAttacker(k, sl, atk)
	// The AtEOT$ ExileCombat promise (gorge scheduleAtEOTRider __kwWarpExile
	// at this turn's end-of-combat step): the record is self-sourced (no
	// creator; gorge registers the token itself, no |CR=), incarnation-
	// tracked like the warp body, and fireDlyEndCombat enqueues it as the
	// step begins. fireDelayed drops one still pooled at the turn's end step
	// (gorge's |TT= expiry); a promise whose copy died in combat still
	// enqueues and resolves doing nothing (CR 603.7).
	if s.ndly < MaxDelayed {
		s.dly[s.ndly] = delayed{act: uint8(kop.OpExile), ctrl: s.res.e.ctrl,
			obj: t, ver: objWord(s.objs[t].ver), when: dlyEndCombat, creator: NoObj}
		s.ndly++
	} else {
		s.capFull(StallDelayed)
	}
	s.touchSl(sl)
}

// myriadProg reports whether prog is a Myriad trigger's whole body, the one
// ExtMyriad: a keyword's expansion, which a copy of a copy has once
// (copyGrantOnce).
func myriadProg(prog []kop.Instr) bool {
	return len(prog) == 1 && prog[0].Op == kop.OpExt && prog[0].Ext() == kop.ExtMyriad
}
