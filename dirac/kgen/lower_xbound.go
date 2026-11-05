package kgen

import "fmt"

// kxpaid: Count$xPaid read by a triggered ability that paid no {X} of its
// own. gorge (rules/stack.go resolveTop: ctx.X = o.X, then triggerPaidX)
// reads the X captured when the trigger matched off the causing event's
// card (trigger_referents.go: c.TriggerPaidX = card.X), and failing that
// the source's live X (effects/count_body.go evalCountBodyCostXPaid). The
// kernel captures the first (pendingTrig.x, Tf2XBound) for the two shapes
// whose trigger object is a spell or the permanent a resolving spell
// becomes, and the second is 0 there:
//
//   - ETB of the card itself (not a batch, no kicker {X}): the entering
//     object; the
//     source is that object, so its live X is the captured one;
//   - SpellCast (not or-copy): the cast spell; when the trigger is not
//     the spell's own, the source must have no {X} in its costs, or its
//     live X could stand in for a captured 0. The spell's own cast
//     trigger (Card.Self; kernel fireSelfCast) reads the spell's paid X.
//
// A self-enters trigger's condition reads the same X (Ravenous): as the object
// enters X reads the resolving spell's, and on the recheck the bound one.
//
// Anything else refuses: any other trigger condition reading X (it is
// checked before the capture exists), an attack, death, cycle or delayed
// trigger reading it (gorge reads the source's live or cleared X).
func (lc *lowerCtx) xBound() error {
	tr := lc.ab.Trigger
	if tr != nil && lc.adamantTrigger() != nil && (lc.out.Kicker == nil || !lc.out.Kicker.X) {
		tr.XBound = true
		return nil
	}
	if tr == nil || !lc.trigBody || lc.ab.Latent {
		return fmt.Errorf("Count$xPaid in a trigger without an {X} cost")
	}
	if lc.out.Kicker != nil && lc.out.Kicker.X {
		// a kicked {X} cast (Verdeloth the Ancient, Kangee): its X is paid
		// through the kicker, which the lockstep showed gorge's
		// lowering surface pays otherwise (it taps for the kicker before
		// the X ask): not lifted here
		return fmt.Errorf("Count$xPaid in a trigger without an {X} cost (kicker {X})")
	}
	self := tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0
	switch {
	case tr.Event == "ETB" && !tr.Batch && self:
	case tr.Event == "SpellCast" && !tr.OrCopy && (self || !lc.cardHasX()):
		// the card's own cast trigger (selfcast): the spell is the
		// trigger object and the source alike, so its paid X is the
		// captured one
	case tr.Event == "Discarded" && tr.Cycled && self:
		// the card's own cycle trigger (Mode$ Cycled ValidCard$ Card.Self):
		// the X announced for the cycling ability, carried by the kernel's
		// fireCycled (gorge trigger_referents.go e.cast.x)
	case tr.Event == "TurnFaceUp" && !tr.Batch && self:
		// the card's own turn-up trigger: the X announced for the Morph-family
		// keyword cost (CR 107.3m), carried by the kernel's fireTurnFaceUp
		// (gorge morph_turnup.go TurnFaceUp Amount, TriggerPaidX)
	default:
		return fmt.Errorf("Count$xPaid in a trigger without an {X} cost (%s trigger)", tr.Event)
	}
	tr.XBound = true
	return nil
}

// spentBound: Count$CastTotalManaSpent read by the body of a self-ETB
// trigger (Astelli Reclaimer). gorge reads manaSpentTotalsOf of the source
// object (count_body_paid.go), the cast spend carried stack to battlefield
// and 0 for a permanent that was not cast; the kernel copies the entering
// spell's recorded spend into the pending entry as it triggers
// (Tf2SpentBound). Every other trigger refuses.
func (lc *lowerCtx) spentBound() error {
	if lc.ab == nil || lc.ab.Trigger == nil || !lc.trigBody || lc.ab.Latent {
		return fmt.Errorf("CastTotalManaSpent outside an as-enters copy")
	}
	tr := lc.ab.Trigger
	self := tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0
	if tr.Event != "ETB" || tr.Batch || !self {
		return fmt.Errorf("CastTotalManaSpent in a %s trigger", tr.Event)
	}
	tr.SpentBound = true
	return nil
}

// cardHasX: the card's own mana cost or kicker has an {X}.
func (lc *lowerCtx) cardHasX() bool {
	return lc.out.Cost.X || lc.out.Kicker != nil && lc.out.Kicker.X
}
