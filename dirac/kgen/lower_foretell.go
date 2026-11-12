package kgen

// K:Foretell:<cost> (CR 702.143a; dirac/kw_foretell.go).

// refuseForetellCombos refuses foretell beside any other cast cost route
// (the kernel's foretold cast pays one plain mana cost from exile), on an
// {X} spell or on a multi-face card (neither the {2} action nor the exile
// cast has a face route). The keyword itself is lowered by lowerKeywords; a
// card refused here keeps only its static record.
func (lc *lowerCtx) refuseForetellCombos() {
	out := lc.out
	if out.Foretell == nil {
		return
	}
	if out.Kicker != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil || out.Convoke || out.Improvise ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil || out.Mayhem != nil ||
		out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree || out.WebSling != nil ||
		out.Evoke != nil || out.Dash != nil || out.MayFlash != nil || out.Paradigm || out.Suspend != nil {
		lc.refuseNode("K:Foretell", "Foretell beside an alternative or additional cost")
	}
	if out.Cost.X {
		lc.refuseNode("K:Foretell", "Foretell on an {X} spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Foretell", "Foretell on a multi-face card")
	}
}
