package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Web-slinging (CR 702.188), cold: reached only when the
// table has a card with printed web-slinging (genPrep.anyWebSling).
//
// A hand card with web-slinging may be cast for its web-slinging cost plus
// returning a tapped creature its caster controls to its owner's hand
// (gorge rules webslinging.go: the parameter replaces the mana cost and is
// composed with Return<1/Creature.YouCtrl+tapped>; legal_walk_hand.go's
// "web-slinging" cast option). It is offered under the plain cast's timing
// and prohibition gates (no timing rider of its own). The cast is
// AkWebSling; its provenance rides the spell entry's trigPl =
// trigPlWebSling (0 B), which a copy does not keep (CR 707.10: gorge's
// FlagWebSlinged is cast provenance; mintSpellCopy). It is read by
// buildCost, the cost stage and the webSlinged word on the entering
// permanent (PredWasCastEnter bit 16, cast_wascastenter.go wasCastRead).
//
// The return is sneak's (kw_sneak.go sneakReturnOptions over the
// activation Return machinery, act_energy.go pkReturn / ChCostReturn /
// payReturnPicks): one pick, asked even with one candidate, paid at
// payCast after the mana and before the sacrifices, so a reversed cast
// moves nothing (CR 733.1). There is no entry rider.
//
// gorge's planner prices no "web-slinging" cast (potential_plan.go
// potentialModeBaseCost has no arm), so under the unpriced surface the
// option exists only when the floating pool pays the adjusted cost, the
// harmonize cast's rule (kw_harmonize.go), and it is never
// lowered.

// trigPlWebSling (trigpl.go) marks a spell entry cast for its web-slinging
// cost.

// DebugWebSlingUntapped lets an untapped creature pay the return
// (mutation testing only: the lockstep must catch it).
var DebugWebSlingUntapped bool

// webSlingEntry reports whether e is a spell cast for its web-slinging
// cost.
func webSlingEntry(e *stackEntry) bool {
	return e.kind == seSpell && e.trigPl == trigPlWebSling
}

// webSlingReturnable reports whether slot sl (controlled by the payer) can
// be returned for a web-slinging cost: a creature, tapped (gorge
// "Creature.YouCtrl+tapped"), not already committed to another cost pick.
// The spell itself is on the stack, never a candidate.
func (s *State) webSlingReturnable(k *prep, sl Slot) bool {
	pp := &s.perms[sl]
	return (pp.flags&pfTapped != 0 || DebugWebSlingUntapped) && s.typesGen(k, sl)&kop.TCreature != 0 && !s.picked(pp.obj)
}

// altReturnable is the return pick's candidate test for spell entry e: the
// web-slinging cast's or the sneak cast's.
func (s *State) altReturnable(k *prep, sl Slot, e *stackEntry) bool {
	if webSlingEntry(e) {
		return s.webSlingReturnable(k, sl)
	}
	return s.sneakReturnable(k, sl)
}

// webSlingHasReturn reports whether p controls a creature the return can
// take (gorge offerCastable's nonManaCastable Return census).
func (s *State) webSlingHasReturn(k *prep, p uint8) bool {
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		if s.webSlingReturnable(k, m.first()) {
			return true
		}
	}
	return false
}

// offerWebSlingHand offers hand card o's web-slinging cast (gorge
// legal_walk_hand.go). The caller checked the plain cast's timing, its
// prohibition and that the card has web-slinging.
//
//go:noinline
func (s *State) offerWebSlingHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.castHeldOut(o) || !s.webSlingHasReturn(k, p) || !s.spellTargetsFeasible(k, c, p) {
		return // castSuppressed; offerCastable's Return census; castTargetsAvailable
	}
	// the avail is the caller's walk picture: the cast's own delve credit
	// prices its twobrid faces and is restored after (offerPlay's shape)
	saved := av.delve
	defer func() { av.delve = saved }()
	av.delve = s.tbDelveCredit(k, p, c, o, true) // the faces' credit (CR 702.66a)
	cost := s.adjustCost(k, p, c, o, k.t.Cards[c].WebSlingCost)
	if !av.tbLeafCredited(&cost) {
		cost = s.delveOfferCredit(k, p, c, o, true, cost) // gorge's choke point credits the generic at this walk too
	}
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
		return // castRestrictedCost / offerCastable on the composed cost
	}
	// gorge's potential walk prices no web-slinging mode; the restricted-grant
	// filter (walkHandAvail) is not this offer's shape.
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.sneakPoolPays(k, p, c, o, &cost) {
		// gorge's planner and lowering price no web-slinging mode: the
		// cast is offered only when the floating pool pays (harmonize's
		// rule)
		return
	}
	d.add(MakeAction(AkWebSling, uint8(o), 0, 0))
}

// beginWebSling casts hand card o for its web-slinging cost (gorge
// cast_begin.go's "web-slinging" mode): an ordinary cast, never lowered
// (castFace keeps it out of beginCast's lowering), marked so buildCost
// prices the web-slinging cost and the cost stage asks the return.
//
//go:noinline
func (s *State) beginWebSling(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.trigPl = trigPlWebSling
}
