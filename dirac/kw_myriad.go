package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Myriad (CR 702.116a; gorge effMyriad, effects/myriad.go): "whenever this
// creature attacks, for each opponent other than defending player, you may
// create a token that's a copy of this creature that's tapped and attacking
// that player or a planeswalker they control. Exile the tokens at end of
// combat." ExtMyriad (kgen lower_myriad.go) is the trigger's whole body.
//
// The opponents are the living players from the controller in turn order
// (gorge AliveFrom) but the controller and the defending player, the one the
// trigger's attack event named as it was queued (stackEntry.defSeat; gorge
// carries the DeclareAttackers event's Player in the trigger's Remembered).
// The controller is asked about each in turn (raGenMyriad, ChGenMyriad: gorge
// poses a KChoose of "yes" / "no" over the source, resumed with the next
// opponent); res.left holds the opponent asked about. A yes makes the copy
// op's token copies of the source (Obj, read as OpCopyAsTokenEx reads Obj, B
// and C; CpTapped) at once, each entering attacking that player: gorge's
// MyriadCopy names the player alone and asks no planeswalker defender, so the
// kernel stalls on nothing here. A copy that parks an ask stops the game
// (StallCopyAllAsk, as kw_encore.go).
//
// The copies are marked (mechMyriad) and exiled as the combat phase ends,
// before the combat state resets, with no trigger (gorge turn.go's
// MyriadCleanup emit as the end of combat step is left: events
// foldMyriadCleanup moves each marked token still on the battlefield to exile
// with TriggerNone).

// DebugMyriadNoOpp is the mutation switch: no opponent is asked about, the
// old two-seat reading ("no opponent other than the defending player").
// Tests only.
var DebugMyriadNoOpp bool

// myriadExt is OpExt ExtMyriad: the first opponent's ask, or nothing.
//
//kmanifest:ext ExtMyriad
func (s *State) myriadExt(k *prep, in *kop.Instr) (ok, done bool) {
	return !s.myriadAsk(k, noSeat), true
}

// myriadAsk asks about the next opponent after seat after (noSeat: the
// first); false when none is left.
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

// myriadOptions poses gorge's options over the source: yes, then no.
func (s *State) myriadOptions(d *Decision) {
	d.Sub = ChGenMyriad
	src := TCode(s.res.e.obj)
	d.add(MakeAction(AkChoose, src, 0, 1))
	d.add(MakeAction(AkChoose, src, 0, 0))
}

// myriadAnswer makes the copy on a yes, then asks about the next opponent.
func (s *State) myriadAnswer(k *prep, a Action) {
	r := &s.res
	opp := r.left
	r.ask = raNone
	if a.Mode() == 1 {
		s.myriadCopy(k, &s.prog(k)[r.pc], opp)
		if s.over {
			return
		}
	}
	if !s.myriadAsk(k, opp) {
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
	s.encoreCopy(k, &cin, c, opp) // kw_encore.go: the per-opponent copy, entering under rfOppCopy
	s.res.flags &^= rfCopyGrp
}

// myriadEnter is a Myriad copy's entry: tapped (CpTapped) and attacking the
// opponent res.left, marked for the end of combat.
func (s *State) myriadEnter(k *prep, sl Slot) {
	s.setEnterAttacker(k, sl, PlayerCode(s.res.left))
	s.mSlSet(k, mechMyriad, sl)
	s.touchSl(sl)
}

// myriadEndCombat exiles every marked token still on the battlefield as the
// combat ends, with no trigger (gorge foldMyriadCleanup).
func (s *State) myriadEndCombat(k *prep) {
	m := s.mSlMask(k, mechMyriad).and(s.occ)
	for m.any() {
		sl := m.first()
		m = m.rest()
		s.mSlClr(k, mechMyriad, sl)
		s.removePerm(k, sl, kop.ZoneExile, false)
	}
}

// myriadProg reports whether prog is a Myriad trigger's whole body, the one
// ExtMyriad: a keyword's expansion, which a copy of a copy has once
// (copyGrantOnce).
func myriadProg(prog []kop.Instr) bool {
	return len(prog) == 1 && prog[0].Op == kop.OpExt && prog[0].Ext() == kop.ExtMyriad
}
