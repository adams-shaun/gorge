package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// returnSelfDeferred pays a CostReturnSelf the mana stage held back (the
// source could tap for mana: gorge taps the planned sources before any
// cost is paid, CR 601.2g-h): the source returns to its owner's hand.
//
//go:noinline
func (s *State) returnSelfDeferred(k *prep, e *stackEntry) {
	if ob := &s.objs[e.obj]; ob.zone == kop.ZoneBattlefield && ob.ver == e.srcVer {
		z := kop.ZoneHand
		if k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostExileSelf != 0 {
			z = kop.ZoneExile // CostExileSelf (trig_delayedtiming.go exileSelfCost)
		}
		s.leave(k, ob.slot, z, false)
	}
}

// countReturn counts the permanents p controls that a PayReturn cost
// matching f could return (the candidates resolveOptionsGen offers; gorge
// UnlessCandidatesFor walks the payer's battlefield, the source included).
//
//go:noinline
func (s *State) countReturn(k *prep, p uint8, f kop.FilterID) int {
	src := s.resSrcSlot(k)
	n := 0
	for m := s.ctrlM[SeatIx(p)].andNot(s.g.sacM); m.any(); m = m.rest() {
		if s.matchPermGen(k, f, m.first(), p, src, layerFull) {
			n++
		}
	}
	return n
}

// pkReturn is a Return<N/Spec> cost pick (kop.CostReturn is above the
// pickKind range: it takes CostUntap's value, never a pick kind, plus
// pkExile's bit so applyBuild routes the answer to answerCostObj).
const pkReturn = pickKind(kop.CostUntap) | pkExile

// returnCands counts (and, with d, offers) p's battlefield permanents
// matching f not already picked (gorge returnAsk / castparts: the payer's
// battlefield, the source included unless the spec says Other).
func (s *State) returnCands(k *prep, p uint8, f kop.FilterID, src Obj, d *Decision) int {
	srcSl := NoSlot
	if src != NoObj && s.objs[src].zone == kop.ZoneBattlefield {
		srcSl = s.objs[src].slot
	}
	n := 0
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		o := s.perms[sl].obj
		if s.picked(o) || !s.matchPerm(k, f, sl, p, srcSl) {
			continue
		}
		if k.gen != nil && k.gen.anyBeamBar && src != NoObj {
			granted := s.g.beamPick || s.ctl == ctlBuild && s.bld().altAdd == buildBeamGrant
			if (granted || k.t.Cards[s.objs[src].card].BeamMeUp) && s.beamBarred(k, sl, p, granted) {
				continue
			}
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// returnPickOptions poses the next Return<N/Spec> pick (gorge returnAsk:
// one KChoose of N there, one pick per permanent here; asked even with
// one candidate). False when every one is picked.
//
//go:noinline
func (s *State) returnPickOptions(k *prep, d *Decision, pc *kop.Cost) bool {
	if pc.Flags&kop.CostReturn == 0 {
		return false
	}
	e := &s.bld().e
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkReturn {
			n++
		}
	}
	left := int(pc.N) - n
	if left <= 0 {
		return false
	}
	d.N = 0
	if s.returnCands(k, e.ctrl, pc.Filter, e.obj, d) < left {
		panic("kernel: return cost no longer payable") // gorge aborts; unreachable after the offer gate
	}
	s.bld().pickK, s.bld().pickLeft = pkReturn, uint8(left)
	d.Kind, d.Sub, d.Player = DkChoose, ChCostReturn, e.ctrl
	return true
}

// payReturnResume finishes an unless-cost PayReturn payment that suspended at
// a CR 903.9b commander_zone park. cmdZoneAnswer has consumed the answer, so
// the parked permanent moves to its owner's hand now; the remaining picks are
// paid or asked, then the payment completes and the resolution continues with
// the resolving entry still on the stack (CR 608.2n). Every such park carries
// the resolving entry: the payment runs inside its resolution, not after it.
func (s *State) payReturnResume(k *prep) {
	o := s.g.pre.obj
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield {
		s.leave(k, ob.slot, kop.ZoneHand, false)
		if s.res.ask == raGenCmdZone {
			s.res.flags |= rfPayPark // parked again on another of this payment's moves
			return
		}
	}
	s.g.pickLeft--
	s.payReturnAfter(k)
}

// payReturnAfter continues a PayReturn payment whose current pick is settled:
// it records the remaining unasked picks when exactly they are owed (gorge
// picks them unasked, pay/unlessflow.go AdvanceUnless), else leaves the pick
// ask posed, and finishes the payment when no pick remains.
func (s *State) payReturnAfter(k *prep) {
	if s.g.pickLeft > 0 {
		s.res.ask, s.res.askP = raGenPayPick, s.g.pickP
		if s.autoPayPick(k) {
			if s.res.flags&rfPayPark != 0 {
				return // another commander parked; its answer resumes this payment
			}
			s.payReturnDone(k)
		}
		return
	}
	s.payReturnDone(k)
}

// payReturnDone commits the settled PayReturn payment and continues the
// resolution (the sacrifice riders and the body after it).
func (s *State) payReturnDone(k *prep) {
	s.payCommitSacs(k)
	if s.payReturnPark() {
		return
	}
	s.payDone(k, true)
}

// payReturnPicks returns the chosen permanents to their owners' hands
// (paySacsGen: after the mana, before the sacrifices, as gorge payCast).
func (s *State) payReturnPicks() {
	k := s.k()
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkReturn {
			continue
		}
		if ob := &s.objs[s.bld().picks[i]]; ob.zone == kop.ZoneBattlefield {
			s.leave(k, ob.slot, kop.ZoneHand, false)
		}
	}
}
