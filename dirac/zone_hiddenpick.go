package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// handTakeAll is a required hand move's no-choice arm (gorge
// handMoveOwnersWalk: a required move whose eligible cards number no more
// than the bound takes them all, in hand order, without an ask). It
// reports whether it took them.
//
//go:noinline
func (s *State) handTakeAll(k *prep) bool {
	var d Decision
	s.searchCands(k, &d)
	if int(d.N) > int(s.g.pickLeft) {
		return false
	}
	for i := 0; i < int(d.N); i++ {
		s.resArm().look[s.g.nlook] = Obj(d.Options[i].Source())
		s.g.nlook++
	}
	s.finishSearchNow(k)
	return true
}

// battlefieldCands offers a hidden battlefield pick's candidates (gorge
// effHiddenPick's game-wide pool over a public origin: every player's
// battlefield in seat order, each in its list order, matching the filter
// with the resolving controller as "you"), the ones already picked left
// out.
//
//go:noinline
func (s *State) battlefieldCands(k *prep, in *kop.Instr, d *Decision) {
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		if s.g.pickF&kop.SrOwnZone != 0 && p != s.g.pickP {
			continue // DefinedPlayer$: that player's battlefield only
		}
		n = s.battlefieldOrder(p, s.ctrlM[p], ord[:], n)
	}
	src := s.resSrcSlot(k)
	for _, sl := range ord[:n] {
		o := s.perms[sl].obj
		if s.picksHas(o) {
			continue
		}
		if s.matchPermGen(k, in.F, sl, s.res.e.ctrl, src, layerFull) {
			d.add(MakeAction(AkChoose, uint8(o), 0, 0))
		}
	}
}

// pickLeaves moves a picked permanent from the battlefield to zone z
// (gorge settleChangeZoneMoveAs: one move per picked card, in pick order),
// remembering it with SrRemember. parked reports a CR 903.9b commander move
// that waits for its owner's answer (cmdZoneReplace): the card is still on
// the battlefield and is neither remembered nor dropped from the look list
// until the answered move happens.
//
//go:noinline
func (s *State) pickLeaves(k *prep, o Obj, z kop.Zone) (parked bool) {
	s.leave(k, s.objs[o].slot, z, false)
	if p := &s.g.pre; p.kind == akCmdZone && p.state == preCmdZone && p.obj == o && s.res.ask == raGenCmdZone {
		return true
	}
	if s.g.pickF&kop.SrRemember != 0 {
		s.remember(o)
	}
	s.forgetFound(k, o)
	return false
}

// chooserOf is who answers a search / choose-move over player p's zone:
// p, or the resolving controller with SrChooserYou (gorge Chooser$ You), or
// with SrChooserOpp the resolving controller's first living opponent (gorge
// searchChooser / hiddenPickChooser: Chooser$ Opponent through chooserPlayer;
// read from B, pickF's bit 0 is SrTapped). A ChooseMove reads the bit beside
// its other B readers: over a hand pool bit 0 is the UntilLeaves hand-exile
// link instead, and kgen never lowers Chooser$ Opponent on a hand pick (its
// hand moves admit only Chooser$ You), so the hand pool keeps the link's
// meaning. An unbound referent keeps the fetch player, as gorge's
// chooserPlayer falls back to the owner.
func (s *State) chooserOf(k *prep, p uint8) uint8 {
	if s.g.pickF&kop.SrChooserYou != 0 {
		return s.res.e.ctrl
	}
	if in := &s.prog(k)[s.res.pc]; (in.Op == kop.OpSearchEx || in.Op == kop.OpChooseMove) && in.B.N&kop.SrChooserOpp != 0 &&
		!(in.Op == kop.OpChooseMove && s.searchZone(k) == kop.ZoneHand) {
		if opp := s.playerRef(k, kop.RefOpponent); opp != 255 {
			return opp
		}
	}
	return p
}

// anyCand reports whether the search / choose-move in progress has a
// candidate (a reveal's Optional$ ask is posed only then: gorge effReveal
// skips a target whose pool is empty before asking).
func (s *State) anyCand(k *prep) bool {
	var d Decision
	s.searchCands(k, &d)
	return d.N > 0
}
