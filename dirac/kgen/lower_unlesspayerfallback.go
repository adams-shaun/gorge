package kgen

// unlessPayerFallback is an UnlessCost$ link's payer when the head's one
// target slot may answer empty. gorge's unlessProceed (effects/unless.go)
// reads the default TargetedController selector over the resolving context's
// targets; a link that declared no target of its own reads the head's, and an
// empty answer there falls back to the resolving controller
// (unlessPayerControllerFallback), who is asked (decline only when the cost
// names the targeted object). Star Athlete's attack trigger with no target
// chosen: gorge asks its controller whether to sacrifice. A link with its own
// targets keeps the plain slot controller: gorge's non-nil empty ownTargets
// is the unpaid orientation with no ask.
func (lc *lowerCtx) unlessPayerFallback(up string, payer lRef, nTgt int) lRef {
	if up != "TargetedController" || lc.head || payer.Kind != rTargetController || payer.N != 0 {
		return payer
	}
	if nTgt != 1 || len(lc.ab.Targets) != 1 {
		return payer // the link added a slot of its own, or the head holds several
	}
	if mn := lc.ab.Targets[0].Min; !mn.isConst() || mn.N != 0 {
		return payer // a mandatory target: never empty at resolution
	}
	return lRef{Kind: rTgtCtrlOrYou}
}
