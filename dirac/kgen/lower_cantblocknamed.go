package kgen

// cantBlockNamedBlocker lowers the curse spelling of the CantBlockBy Effect
// ("target creature can't block this creature this turn": Duct Crawler,
// Screeching Griffin, Spin Engine, Kozilek's Pathfinder, Shrewd Hatchling,
// Auriok Siege Sled): ValidAttacker$ Creature.EffectSource, ValidBlocker$
// Creature.IsRemembered over the ability's own target, ForgetOnMoved$
// Battlefield and Duration$ UntilHostLeavesPlayOrEOT. The kernel record is
// the (source, target) pair at their current object versions (dirac
// exkNoBlock), which ends when either leaves the battlefield and at cleanup,
// as gorge's forget sweep and duration do. Any other spelling refuses.
func (lc *lowerCtx) cantBlockNamedBlocker(ps *params, sp *params, obj lRef) error {
	if sp.str("ValidBlocker") != "Creature.IsRemembered" || obj.Kind != rTarget {
		return refuse("Effect CantBlockBy shape")
	}
	switch ps.str("RememberObjects") {
	case "ThisTargetedCard", "Targeted":
	default:
		return refuse("Effect CantBlockBy shape")
	}
	if _, ok := ps.get("ExileOnMoved"); ok || !ps.is("ForgetOnMoved", "Battlefield") ||
		!ps.is("Duration", "UntilHostLeavesPlayOrEOT") {
		return refuse("Effect CantBlockBy shape")
	}
	lc.emit(lInstr{Op: "ExHaste", Obj: obj, W: lRef{Kind: rSelf}, Note: "until end of turn: the target can't block the source (named-blocker record)"})
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect static unread %v", left)
	}
	return nil
}
