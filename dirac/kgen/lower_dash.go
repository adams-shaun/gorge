package kgen

// K:Dash:<cost> (CR 702.109a; dirac/kw_dash.go).

// refuseDashCombos refuses dash beside any other cast cost route (the
// kernel's dashed cast pays one plain alternative mana cost of a
// single-face creature card), on an {X} spell or on a multi-face card, the
// refuseEvokeCombos shape. The dashed provenance shares stackEntry.mode's
// impending bit, so a card with both would be ambiguous.
func (lc *lowerCtx) refuseDashCombos() {
	out := lc.out
	if out.Dash == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.MayFlash != nil {
		lc.refuseNode("K:Dash", "Dash beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Dash", "Dash on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Dash", "Dash on a multi-face card")
	}
}
