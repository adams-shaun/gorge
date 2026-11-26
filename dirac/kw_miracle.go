package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// MiracleGrantStats counts the reveal triggers a granted miracle (a
// StatMiracleGrant static, miracleCostOf) queued for a card without printed
// miracle.
var MiracleGrantStats struct{ Reveals atomic.Int64 }

// Miracle (CR 702.94a; gorge rules/miracle.go, trigger_queue.go,
// stack.go): the first card a player draws in a turn (never the opening
// deal), when it has printed miracle, queues a kernel-native keyword
// trigger (ab abMiracle, source and object the card; gorge's
// "__kwMiracleCast"). Placing it is the reveal choice: the owner is asked
// (ChOptional, gorge's placement KTriggerOptional) before the trigger goes on
// the stack, and a no drops it. A yes puts it on the stack, so both players
// get priority with it there. As it resolves its owner is asked (gorge
// askMiracleCast, ChOptional) to cast the card from hand for its miracle
// cost -- "yes" offered only when the card is castable. The cast is begun once
// the trigger has left the stack, ignoring timing; it carries trigPlMiracle,
// which prices it at CardDef.MiracleCost and pays it through gorge's mana
// window as a madness cast is (a cast begun mid-resolution has no payment
// witness). Declining leaves the card in hand.
const abMiracle uint8 = 249

// trigPlMiracle is in trigpl.go.

const raGenMiracle = raGenManifestDread + 51 // the miracle cast-or-keep ask (ChOptional)

// queueMiracle queues the miracle trigger for card o just drawn as its
// controller's first draw of the turn.
//
//go:noinline
func (s *State) queueMiracle(o Obj) {
	k := s.k()
	ob := &s.objs[o]
	if k.gen == nil || !k.gen.anyMiracle {
		return
	}
	if _, ok := s.miracleCostOf(k, o); !ok {
		return
	}
	if !k.t.Cards[ob.card].HasMiracle {
		MiracleGrantStats.Reveals.Add(1)
	}
	p := ob.owner()
	s.enqueue(o, ob.ver, ob.card, abMiracle, p, o, p, 0)
}

// miracleInHand: the entry's card is still the card in its owner's hand
// (gorge: zone hand, controller the offer's).
func (s *State) miracleInHand(o Obj, ver, p uint8) bool {
	ob := &s.objs[o]
	return ob.zone == kop.ZoneHand && ob.ver == ver && ob.owner() == p
}

// miracleRevealOptions poses the reveal ask of the trigger being placed
// (yes, then no, as gorge's askTriggerOptional lists them).
func (s *State) miracleRevealOptions(d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChOptional, s.bld().e.ctrl
	d.add(MakeAction(AkChoose, 0, 0, 1))
	d.add(MakeAction(AkChoose, 0, 0, 0))
}

// answerMiracleReveal applies the reveal answer: a yes with the card still in
// hand puts the trigger on the stack (gorge revealMiracle), anything else
// drops it. Placement of the rest of the queue goes on.
func (s *State) answerMiracleReveal(k *prep, a Action) {
	e := &s.bld().e
	s.ctl = ctlPlaceTriggers
	if a.Mode() == 1 && s.miracleInHand(e.obj, e.srcVer, e.ctrl) {
		s.pushBuild(k)
	}
}

// resolveMiracle poses the cast-or-keep ask, or finishes when the card has
// left the hand (gorge: the ability simply finishes).
//
//go:noinline
func (s *State) resolveMiracle(k *prep) {
	e := &s.res.e
	if !s.miracleInHand(e.obj, e.srcVer, e.ctrl) {
		s.finishResolve(k, false)
		return
	}
	s.res.ask, s.res.askP = raGenMiracle, e.ctrl
	s.ctl = ctlResolve
}

// miracleCastable: the owner can cast hand card o for its miracle cost
// (gorge offerCastablePotential): the cost payable and every target slot
// feasible. Gorge prices the cost against its potential pool, an over-bound
// (a source adds one unit of every colour it can make), so a combo land beside
// an island still "pays" {3}{W} and the yes is offered; the cast then fails to
// pay. The kernel prices the same bound (availPotentialFor) and so offers it.
func (s *State) miracleCastable(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	base, ok := s.miracleCostOf(k, o)
	if !ok {
		return false // a granted miracle whose grant is gone (gorge askMiracleCast: no miracleCost, no ask)
	}
	cost := s.adjustCost(k, p, c, o, base)
	return s.keywordCastAffords(k, p, o, &cost) && s.spellTargetsFeasible(k, c, p)
}

// miracleAskOptions builds the resolution ask's options.
func (s *State) miracleAskOptions(k *prep, d *Decision) bool {
	d.Sub = ChOptional
	if s.miracleCastable(k, s.res.e.obj, s.res.askP) {
		d.add(MakeAction(AkChoose, 0, 0, 1))
	}
	d.add(MakeAction(AkChoose, 0, 0, 0))
	return true
}

// applyMiracleAsk applies the resolution ask: a yes begins the cast, a no
// leaves the card in hand.
func (s *State) applyMiracleAsk(k *prep, a Action) bool {
	s.res.ask = raNone
	o, p, ver := s.res.e.obj, s.res.e.ctrl, s.res.e.srcVer
	s.finishResolve(k, false)
	if a.Mode() == 1 && s.miracleInHand(o, ver, p) && s.miracleCastable(k, o, p) {
		// the caster (beginCast reads it); after the cast the priority
		// holder the resolution left (the active player, CR 117.3b) gets
		// priority, as gorge resumes after the miracle trigger
		holder := s.prio
		s.prio = p
		s.beginCast(k, o, 0, castMiracleTgt) // priced at the miracle cost, no witness lowered
		s.prio = holder
	}
	return true
}

// castMiracleTgt in beginCast's target byte, with mode 0 (from the hand), is
// the miracle cast applyMiracleAsk begins; no AkCast carries it.
const castMiracleTgt uint8 = 0xff

// miracleCastPl is beginCast's trigPl for that cast: trigPlMiracle, so it is
// priced at the miracle cost and lowers no witness from the start; pl
// otherwise.
func (s *State) miracleCastPl(k *prep, o Obj, mode, tgt, pl uint8) uint8 {
	if mode != 0 || tgt != castMiracleTgt {
		return pl
	}
	if _, ok := s.miracleCostOf(k, o); !ok {
		return pl
	}
	return trigPlMiracle
}

// miracleBase is the miracle cost a miracle cast prices each target at
// (cost_reducespell.go): e is a miracle-priced spell entry.
func (s *State) miracleBase(k *prep, e *stackEntry) (kop.ManaCost, bool) {
	if e.trigPl != trigPlMiracle {
		return kop.ManaCost{}, false
	}
	return s.miracleCostOf(k, e.obj)
}

// miracleCostOf is card o's miracle cost (gorge miracleCost): its printed one,
// else the first grant in force (a StatMiracleGrant static, in perm.ts order
// as gorge's derived keyword list is composed), read live; false when it has
// none. The card is read as a card in its owner's hand even while the cast
// has it on the stack: the grant is the hand's, and a cast begun from the
// miracle trigger had it there.
func (s *State) miracleCostOf(k *prep, o Obj) (kop.ManaCost, bool) {
	ob := &s.objs[o]
	c := ob.card
	cd := &k.t.Cards[c]
	if cd.HasMiracle {
		return cd.MiracleCost, true
	}
	if k.gen == nil || !k.gen.anyMiracle {
		return kop.ManaCost{}, false
	}
	var cost kop.ManaCost
	got := false
	mo := o
	if ob.zone != kop.ZoneHand {
		mo = NoObj // on the stack mid-cast: the hand-zone filter reads the printed card (matchCardOneAt's zone test needs the object)
	}
	s.eachStaticTS(k, sclsOffKwG, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatMiracleGrant {
			return true
		}
		if !s.statCond(k, st) || !s.matchCardGenAt(k, a.Static.Filter, c, ob.owner(), st.ctrl, mo, nil) {
			return true
		}
		cost, got = a.Static.GrantCost, true
		if less := a.Static.CardCostLess; less != 0 {
			cost = cd.Cost
			cost.Generic -= min(cost.Generic, less-1) // CR 702.94a "reduced by": never below zero
		}
		return false
	})
	return cost, got
}
