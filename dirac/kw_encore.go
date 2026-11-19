package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Encore (CR 702.141a; gorge effEncore, effects/encore.go): "for each
// opponent, create a token copy of it. Those tokens attack that opponent this
// turn if able. They gain haste. Sacrifice them at the beginning of the next
// end step." ExtEncore (kgen lower_encore.go) walks the opponents from the
// resolving controller in turn order (gorge AliveFrom), one copy op each, read
// as OpCopyAsTokenEx reads Obj, B and C: the haste is its PumpKeywords$ EOT
// grant and the sacrifice its CpSacAtEndDly rider, every copy after the
// first joining the first one's trigger (rfCopyGrp held across the walk: gorge
// registers one DelayedRegister for the whole group). The CreateToken
// replacements are read from the permanents present as the walk begins
// (tokFrz), as gorge sizes every opponent's copies before the first mint.
//
// Each copy's attack duty names its opponent: the arena bits
// mechAttacksIfAble and mechAttacksPlayer as ExtAttacksIfAble's
// AttacksOpponent sets them, and the seat in mechEncSeat0/1 (gorge
// Object.EncoreAttackDefender). encoreSeat reads it back for the attackers
// decision: the copy is offered that player alone, and the duty binds only
// while that player is in the game and the pair can be declared (gorge
// encoreAttackDefender; CR 508.1d).
//
// A copy that parks an ask (an as-enters choice, an Aura's host, a CR 616.1
// entry order) or a CreateToken rewriter stops the game (StallCopyAllAsk,
// StallTokRewrite): the walk keeps no place to resume from.

// DebugEncoreOneCopy is the mutation switch: the first opponent alone gets a
// copy, the old two-seat reading. Tests only.
var DebugEncoreOneCopy bool

// encoreExt is OpExt ExtEncore.
//
//kmanifest:ext ExtEncore
func (s *State) encoreExt(k *prep, in *kop.Instr) (ok, done bool) {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return true, true
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	e := &s.res.e
	q := s.oppsFrom(e.ctrl)
	if DebugEncoreOneCopy && q.n > 1 {
		q.n = 1
	}
	cin := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: in.Obj, A: kop.Const(1), B: in.B, C: in.C}
	s.g.tokFrz, s.g.tokFrzOn = s.statM, true
	s.res.flags &^= rfCopyGrp
	for i := uint8(0); i < q.n && s.g.stalled == 0; i++ {
		s.encoreCopy(k, &cin, c, q.s[i])
	}
	s.g.tokFrzOn = false
	s.res.flags &^= rfCopyGrp | rfOppCopy
	return true, true
}

// encoreCopy makes card c's copies for opponent opp.
func (s *State) encoreCopy(k *prep, in *kop.Instr, c CardID, opp uint8) {
	e := &s.res.e
	_, n, rewritten, ok := s.copyTokenRewrite(k, e.ctrl, c, 1, true)
	switch {
	case !ok:
		return // StallTokRewrite
	case rewritten:
		s.stall(StallTokRewrite)
		return
	}
	if s.mintTokAdds(k, &kop.Instr{Op: kop.OpCreateToken}, e.ctrl, c, n) || s.g.stalled != 0 {
		s.stall(StallCopyAllAsk)
		return
	}
	s.res.flags |= rfOppCopy
	s.res.left = opp
	if s.copyTokensRun(k, in, c, n) {
		s.stall(StallCopyAllAsk)
	}
	s.res.flags &^= rfOppCopy
}

// oppCopyEnter is a per-opponent copy's entry (copyTokenEnterFrom, under
// rfOppCopy): an Encore copy's attack duty names the opponent s.res.left;
// a Myriad copy (kw_myriad.go) enters attacking that player.
func (s *State) oppCopyEnter(k *prep, in *kop.Instr, sl Slot) {
	if s.res.e.kind == seTrigger {
		s.myriadEnter(k, sl) // kw_myriad.go: Myriad is the trigger's copy
		return
	}
	opp := s.res.left
	s.mSlSet(k, mechAttacksIfAble, sl)
	s.mSlSet(k, mechAttacksPlayer, sl)
	s.mSlClr(k, mechEncSeat0, sl)
	s.mSlClr(k, mechEncSeat1, sl)
	if opp&1 != 0 { // seat-ok: the seat's bit 0 in its own mask
		s.mSlSet(k, mechEncSeat0, sl)
	}
	if opp&2 != 0 { // seat-ok: the seat's bit 1 in its own mask
		s.mSlSet(k, mechEncSeat1, sl)
	}
	s.touchSl(sl)
}

// encoreSeat is the opponent the Encore duty on sl names, and whether the
// duty binds: that player is in the game and the pair can be declared (gorge
// encoreAttackDefender's CanAttackPair).
func (s *State) encoreSeat(k *prep, sl Slot) (uint8, bool) {
	if k.gen == nil || k.gen.ix[mechEncSeat0] == arenaNone || !s.mSl(k, mechAttacksPlayer, sl) {
		return noSeat, false
	}
	q := uint8(0)
	if s.mSl(k, mechEncSeat0, sl) {
		q |= 1
	}
	if s.mSl(k, mechEncSeat1, sl) {
		q |= 2
	}
	if q >= s.nSeats() || q == s.active || s.out(q) || s.cantAtkPairPlayer(k, sl, q) ||
		s.atkHasteSick(k, sl) && !s.atkHasteOK(k, sl, q) {
		return q, false
	}
	return q, true
}

// encoreUses reports the arena bits ExtEncore, ExtGrantEncore
// (kw_encoregrant.go) and ExtMyriad (kw_myriad.go) reach (arenaUses).
func encoreUses(in *kop.Instr, u *[numMech]bool) {
	switch {
	case in.Op != kop.OpExt:
	case in.Ext() == kop.ExtGrantEncore:
		u[mechEncore] = true
	case in.Ext() == kop.ExtMyriad:
		u[mechMyriad] = true
	case in.Ext() == kop.ExtEncore:
		u[mechAttacksIfAble], u[mechAttacksPlayer] = true, true
		u[mechEncSeat0], u[mechEncSeat1] = true, true
	}
}
