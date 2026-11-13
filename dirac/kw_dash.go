package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Dash (CR 702.109a): the hand offers an alternative mana cost (gorge's
// "dashed" cast option, altcast_modes.go altDash: a potential play, offered
// like warp, impending and evoke before the plain affordability gate). It rides
// AkCast mode 0 with castDashTgt in the target byte; legal.go sets
// genState.impCast and beginCast marks the build entry with modeImpended (a
// card has printed dash, impending or neither, never two: kgen refuses the
// combination, so the bit reads as "dashed" only on a card with HasDash).
// buildCost prices DashCost; the cast is lowered like a warp cast (cast.go).
// A stack copy clears the mode bit (copy_gen.go), so a copy is not dashed.
//
// The permanent that cast produces gains haste until end of turn and returns
// to its owner's hand at the beginning of the next end step if it is still the
// same permanent (gorge altcast.go altCostEnter: an UntilEOT haste grant and
// the incarnation-tracked __kwDashReturn delayed body). The return is a
// delayed record whose act is the kernel-internal opDashReturn, ordered with
// the other end-step triggers even when stale (staleOrdered), resolving as a
// bounce of the registered incarnation only.

// castDashTgt in an AkCast's target byte (mode 0, from the hand) is the cast
// for the card's dash cost (offerDashHand).
const castDashTgt = 4

// opDashReturn is the delayed record's act and the end-step trigger's trigAmt
// for the dash return (kernel-internal, beside opLeaveReturn and
// opSacUntracked; never in an op dispatch).
const opDashReturn = 232

// DebugDashNoReturn skips the dash return registration and DebugDashNoHaste
// the haste grant (TestDashMutation: the lockstep must catch each).
var DebugDashNoReturn, DebugDashNoHaste bool

// dashed reports whether e is a spell cast for its dash cost.
func (e *stackEntry) dashed(k *prep) bool {
	return k.t.Cards[e.card].HasDash && e.impended()
}

// offerDashHand offers hand card o's dashed cast (gorge's "dashed" option).
// The caller has checked the card's timing and the cast prohibition; it runs
// before the plain affordability gate, like evoke's.
//
//go:noinline
func (s *State) offerDashHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	cd := &k.t.Cards[c]
	dc := s.adjustCost(k, p, c, o, cd.DashCost) // the reduction applies to an alternative cost too (CR 601.2f)
	if len(k.gen.grantPC) != 0 && s.g.availX&availWalk != 0 {
		// gorge's potential walk counts a restricted mana ability only for a
		// payment its restriction admits (potentialRestrictionAdmits): a
		// hand-barred grant puts no cast on the walk grantCap reads
		var ha avail
		s.walkHandAvail(k, p, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}, &ha)
		av = &ha
	}
	if !av.affords(&dc, pipMaskOf(&dc), 0) && (!s.rbAny(p) && !k.restrSrc || !s.restrFallback(k, p, c, s.restrOriginOfObj(o), &dc, av)) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &dc, av) {
		return
	}
	if s.castHeldOut(o) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castDashTgt, 0))
}

// dashEnter runs after the resolving permanent spell e entered at slot sl: a
// dashed one gains haste until end of turn and arms its return to hand for the
// beginning of the next end step.
//
//go:noinline
func (s *State) dashEnter(sl Slot, e *stackEntry) {
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	s.touchSl(sl)
	if !DebugDashNoHaste {
		s.grantEOT(sl, kop.KwHaste)
	}
	if DebugDashNoReturn {
		return
	}
	if s.ndly >= MaxDelayed {
		s.capFull(StallDelayed)
		return
	}
	s.dly[s.ndly] = delayed{act: opDashReturn, ctrl: e.ctrl, obj: e.obj, ver: objWord(ob.ver), creator: NoObj}
	s.ndly++
}
