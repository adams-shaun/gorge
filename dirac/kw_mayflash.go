package dirac

// May-flash (CR 702.8, Forge's K:MayFlashCost), cold: reached only when the
// table has a card with the keyword (genPrep.anyMayFlash).
//
// A Behold<1/Type> extra (Molten Exhale, CardDef.MayFlashBehold) adds no
// mana: the cast is offered only while the caster has a candidate (a
// permanent they control or another hand card of the type), and it owes the
// behold in the cost-pick stage after its targets, as gorge's beholdCostAsk
// poses it for the composed cost (act_behold.go beholdOptions).
//
// "You may cast this as though it had flash if you pay {2} more to cast
// it." gorge (rules mayflash.go, legal_walk_hand.go): when the ordinary
// timing gate refuses the card -- not an instant, no flash, no flash grant,
// and not sorcery timing -- the hand offers one cast, Mode "mayflash", whose
// charge is the mana cost plus the keyword's extra (cast_begin.go
// castModeMayflash: base.Plus(extra), never a substitution), under the plain
// cast's prohibition, target and cost-modifier gates. At sorcery timing, or
// with flash from any source, the plain cast is the only one: no may-flash
// option is offered beside it. The cast is AkMayFlash; its provenance rides
// the spell entry's trigPl = trigPlMayFlash (0 B), read only by buildCost.
// gorge records no cast flag for it, so nothing after the cast reads it.
//
// gorge's planner prices no "mayflash" cast (payment_plan.go plans only the
// plain, may-play and flashback casts; potential_plan.go's
// potentialModeBaseCost has no arm), so under a gorge surface the option
// exists only when the floating pool pays the adjusted cost, the
// web-slinging cast's rule (kw_websling.go), and it is never lowered.

// DebugMayFlashFree prices the may-flash cast at the plain mana cost
// (mutation testing only: the lockstep must catch it).
var DebugMayFlashFree bool

// mayFlashEntry reports whether e is a spell cast through its may-flash
// permission.
func mayFlashEntry(e *stackEntry) bool {
	return e.kind == seSpell && e.trigPl == trigPlMayFlash
}

// offerMayFlashHand offers hand card o's may-flash cast (gorge
// legal_walk_hand.go, inside the failed timing gate). The caller checked
// that the card's own timing refuses the plain cast and that it has the
// keyword.
//
//go:noinline
func (s *State) offerMayFlashHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if s.castHeldOut(o) || k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return // gorge castSuppressed / castRestricted
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
		return // castTargetsAvailable; offerCastable's non-mana census
	}
	cd := &k.t.Cards[c]
	if cd.MayFlashBehold && s.beholdCands(k, p, c, o, nil) < int(cd.BeholdN) {
		return // the Behold<1/Type> extra needs a candidate (offerCastable's non-mana census)
	}
	// the avail is the caller's walk picture: the cast's own delve credit
	// prices its twobrid faces and is restored after (offerPlay's shape)
	saved := av.delve
	defer func() { av.delve = saved }()
	av.delve = s.tbDelveCredit(k, p, c, o, true)                        // the faces' credit (CR 702.66a)
	cost := s.adjustCost(k, p, c, o, addCost(cd.Cost, cd.MayFlashCost)) // spellScope("mayflash")
	if !av.tbLeafCredited(&cost) {
		cost = s.delveOfferCredit(k, p, c, o, true, cost)
	}
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) {
		return // offerCastable on the composed cost
	}
	// gorge's potential walk prices no may-flash mode; the restricted-grant
	// filter (walkHandAvail) is not this offer's shape.
	if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.sneakPoolPays(k, p, c, o, &cost) {
		// gorge's planner and lowering price no may-flash mode: the cast
		// is offered only when the floating pool pays (web-slinging's rule)
		return
	}
	d.add(MakeAction(AkMayFlash, uint8(o), 0, 0))
}

// beginMayFlash casts hand card o through its may-flash permission (gorge
// cast_begin.go's "mayflash" mode): an ordinary cast, never lowered
// (castFace keeps it out of beginCast's lowering), marked so buildCost adds
// the extra.
//
//go:noinline
func (s *State) beginMayFlash(k *prep, o Obj) {
	s.beginCast(k, o, castFace, 0)
	s.bld().e.trigPl = trigPlMayFlash
}
