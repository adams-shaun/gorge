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

// DebugDashCmdCasts counts the dashed casts applied from the command zone;
// DebugDashCmdNoOffer skips their offer (TestDashCommandZoneLockstep and
// TestDashCommandZoneMutation: the lockstep must catch the missing offer).
var (
	DebugDashCmdCasts   *int
	DebugDashCmdNoOffer bool
)

// noteDashCmdCast counts a dashed cast action from the command zone (the
// DebugDashCmdCasts test hook).
func (s *State) noteDashCmdCast(a Action) {
	if DebugDashCmdCasts != nil && s.objs[Obj(a.Source())].zone == kop.ZoneCommand {
		*DebugDashCmdCasts++
	}
}

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

// offerDashCommand offers commander o's dashed cast from the command zone
// (gorge's altCastModes loop in the command-zone walk, legal_walk_alt.go): an
// alternative cost replaces the mana cost, not the additional commander tax
// (CR 118.9d, 903.8), so the dash cost is taxed and then reduced as the build
// prices it (buildCostPreRaw). The caller has checked the card's timing and
// the cast prohibition; like the hand offer it runs before the plain
// affordability gate and rides castDashTgt.
//
//go:noinline
func (s *State) offerDashCommand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if DebugDashCmdNoOffer {
		return
	}
	base := k.t.Cards[c].DashCost
	addCommanderTax(&base, s.commanderTax(p, o))
	dc := s.adjustCost(k, p, c, o, base)
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &dc, av) ||
		!s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &dc, av) {
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

// dashReturn returns the dashed permanent at sl to its owner's hand (CR
// 702.109a). A commander's move to a hand is the owner's choice (CR 903.9b):
// leave parks it, and the ask is answered through cmdZoneAnswer, which runs
// dashReturnAnswered for this delayed trigger (it has no program to resume).
func (s *State) dashReturn(k *prep, sl Slot) {
	s.leave(k, sl, kop.ZoneHand, false)
	if s.res.ask == raGenCmdZone {
		s.ctl = ctlResolve // a commander's move parked: its owner answers first
	}
}

// dashReturnAnswered makes the dash return's parked move after the owner's
// CR 903.9b answer (the command zone or the hand) and finishes the trigger.
func (s *State) dashReturnAnswered(k *prep) {
	e := &s.res.e
	ob := &s.objs[dlyRef(e.obj, e.trigObj)]
	if ob.ver == e.srcVer && ob.zone == kop.ZoneBattlefield {
		s.leave(k, ob.slot, kop.ZoneHand, false)
	}
	s.finishResolve(k, false)
}

// dashReturnTrigger reports whether the resolving entry is the dash return.
func (s *State) dashReturnTrigger() bool {
	e := &s.res.e
	return e.kind == seTrigger && e.ab == abDelayed && e.trigAmt == opDashReturn
}
