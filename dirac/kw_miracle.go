package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

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
	if k.gen == nil || !k.gen.anyMiracle || !k.t.Cards[ob.card].HasMiracle {
		return
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
// feasible.
func (s *State) miracleCastable(k *prep, o Obj, p uint8) bool {
	c := s.objs[o].card
	cost := s.adjustCost(k, p, c, o, k.t.Cards[c].MiracleCost)
	return (s.affordsFor(k, p, &cost) || s.windowAffords(k, p, &cost)) && s.spellTargetsFeasible(k, c, p)
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
		s.beginCast(k, o, 0)
		s.prio = holder
		s.bld().e.trigPl = trigPlMiracle
	}
	return true
}
