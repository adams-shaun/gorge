package kgen

// K:Suspend:<N>:<cost> (CR 702.62a; dirac/kw_suspend.go).

// refuseSuspendCombos refuses suspend beside any other cast cost route or
// additional cost (the kernel's suspended cast is a free cast of a plain
// single-face card, and its special action pays one plain mana cost), on an
// {X} spell, or on a multi-face card, the refuseDashCombos shape.
func (lc *lowerCtx) refuseSuspendCombos() {
	out := lc.out
	if out.Suspend == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.MayFlash != nil || out.Dash != nil || out.Paradigm || out.Delve || out.BeamMeUp != nil {
		lc.refuseNode("K:Suspend", "Suspend beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Suspend", "Suspend on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Suspend", "Suspend on a multi-face card")
	}
}
