package kgen

// K:Surge:<cost> (CR 702.117a; dirac/kw_surge.go).

// refuseSurgeCombos refuses surge beside any other cast cost route (the
// kernel's surged cast pays one plain alternative mana cost of a single-face
// card), on an {X} spell, on a multi-face card and on a modal spell, the
// refuseDashCombos shape. The surged provenance shares stackEntry.mode's
// impending bit, which a modal spell's chosen modes also occupy, so a card
// with both would be ambiguous.
func (lc *lowerCtx) refuseSurgeCombos() {
	out := lc.out
	if out.Surge == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.Dash != nil || out.Overload != nil || out.MayFlash != nil || out.Retrace || out.Delve || out.BeamMeUp != nil || out.Escape != nil {
		lc.refuseNode("K:Surge", "Surge beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Surge", "Surge on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Surge", "Surge on a multi-face card")
	}
	for _, a := range out.Abilities {
		if a.Kind == akSpell && a.Modes != 0 {
			lc.refuseNode("K:Surge", "Surge on a modal spell")
			break
		}
	}
}
