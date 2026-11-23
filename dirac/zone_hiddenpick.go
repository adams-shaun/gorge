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
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
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
	pw, addLK := s.pickRemLKPower(k, o) // zone_rememberlki.go: the power before the leave
	s.leave(k, s.objs[o].slot, z, false)
	if p := &s.g.pre; p.kind == akCmdZone && p.state == preCmdZone && p.obj == o && s.res.ask == raGenCmdZone {
		return true
	}
	if addLK {
		s.remLKSum(pw)
	}
	if s.g.pickF&kop.SrRemember != 0 {
		s.remember(o)
	}
	s.forgetFound(k, o)
	s.pickLink(k, o)
	return false
}

// battleLinkPick reports whether in is a battlefield exile pick linked to its
// source (SrBattleLink). Bit 9 of an OpChooseMove's B is also the entry
// counter kind's (kind+1)<<4 field of a graveyard or Hand,Graveyard pick
// (Charnel Serenade's FINALITY sets it), so the bit counts only on the
// zoneless battlefield filter (searchZone) of a pick that is not SrHandGrave.
// The one home of the rule: startSearch, pickLink and tableLinkRet read it here.
func battleLinkPick(t *kop.Table, in *kop.Instr) bool {
	return in.Op == kop.OpChooseMove && in.B.N&kop.SrBattleLink != 0 &&
		in.B.N&kop.SrHandGrave == 0 && t.Filters[in.F].Zone == kop.ZoneNone
}

// pickLink links a battlefield pick's exiled card o to the resolving source
// when the pick is until-leaves (SrBattleLink, Duration$ UntilHostLeavesPlay;
// gorge recordExileReturnFor emits the link for a non-token card that is in
// exile): the card returns to the battlefield under its owner's control when
// the source leaves (returnLinked), the picked set in one entering batch. A
// token is never linked: it ceases instead of returning (CR 111.7), and in a
// generated-table game it still sits in exile at this point, so the zone test
// alone would link it.
//
//go:noinline
func (s *State) pickLink(k *prep, o Obj) {
	pr := s.prog(k)
	if !s.res.active || int(s.res.pc) >= len(pr) || s.objs[o].of&ofToken != 0 {
		return
	}
	if in := &pr[s.res.pc]; !battleLinkPick(k.t, in) || s.objs[o].zone != kop.ZoneExile {
		return
	}
	s.recordExileLink(s.res.e.obj, o)
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

// pickOrigin is the zone the pick in progress comes from, read from the current
// instruction as handGraveArm does: a moveFound driven outside a program
// (kernel tests) has none, and gets ZoneNone, the flag-only entry player.
func (s *State) pickOrigin(k *prep) kop.Zone {
	if pr := s.prog(k); s.res.active && int(s.res.pc) < len(pr) {
		return s.searchZone(k)
	}
	return kop.ZoneNone
}

// pickEntryPlayer is who a picked card o enters the battlefield under, p the
// fetch player, from the pick's origin zone: a pick from a public zone spanning
// every seat -- the game-wide exile pool (from=Exile, zone_exiledwithpick.go)
// and graveexile (SrGraveExile) -- enters under the card's owner unless
// GainControl$ (SrUnderYourControl), as gorge's settleChangeZoneMoveAs passes
// o.Owner (Sengir Nosferatu's stolen Bat returns the exiled Nosferatu under
// its owner, CR 610.3c's family); a GainControl$ pick from a fetch
// player's graveyard (SrOwnZone, The Weaver King) enters under the resolving
// controller, not the fetch player; else the fetch player (the zones whose
// owner owns every card in them: a library, a hand, the fetch player's own
// graveyard); a GainControl$ search of a library enters under the resolving
// controller (gorge settleChangeZoneMoveAs: Bribery searching an opponent's).
func (s *State) pickEntryPlayer(o Obj, p uint8, from kop.Zone) uint8 {
	fl := s.g.pickF
	ep := p
	if (from == kop.ZoneExile || fl&kop.SrGraveExile != 0) && fl&kop.SrUnderYourControl == 0 {
		ep = s.objs[o].owner()
	}
	if fl&(kop.SrUnderYourControl|kop.SrOwnZone) == kop.SrUnderYourControl|kop.SrOwnZone {
		ep = s.res.e.ctrl
	}
	if from == kop.ZoneLibrary && fl&kop.SrUnderYourControl != 0 {
		ep = s.res.e.ctrl // GainControl$ on a search of another player's library (Bribery, Acquire)
	}
	if DebugGyBfOwnerEntry && fl&kop.SrUnderYourControl != 0 {
		ep = s.objs[o].owner() // mutation: a GainControl$ graveyard pick keeps its owner
	}
	return ep
}
