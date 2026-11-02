package kgen

import "strings"

// beamMeUp lowers K:Beam me up:<cost> (gorge beamMeUpCost): the graveyard cast's
// mana is CardDef.AltCost and its returned creature CardDef.FbCost. The bare
// keyword names a grant and is not printed on a card.
func (lc *lowerCtx) beamMeUp(node, param string) {
	mc, err := parseManaCost(param)
	if err != nil || mc.X || mc.None || strings.ContainsAny(param, ":<") {
		lc.refuseNode(node, "beam me up cost %q", param)
		return
	}
	c, err := lc.cost("Return<1/Creature>")
	if err != nil {
		lc.refuseNode(node, "beam me up return: %v", err)
		return
	}
	c.Mana = manaCost{None: true}
	lc.out.BeamMeUp, lc.out.FbCost = &mc, &c
}

// refuseBeamCombos refuses beam me up beside any other cast route or additional
// cost: the cast is one plain alternative mana cost plus the return, and
// CardDef.AltCost and FbCost hold one graveyard cast.
func (lc *lowerCtx) refuseBeamCombos() {
	out := lc.out
	if out.BeamMeUp == nil {
		return
	}
	if out.Flashback != nil || out.Harmonize != nil || out.Mayhem != nil || out.Convoke || out.Improvise || out.Kicker != nil ||
		out.Warp != nil || out.Plot != nil || out.Sneak != nil || out.Impending != nil || out.Bestow != nil || out.SpellCost != nil ||
		out.AltAddCost != nil || out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Casualty != 0 || out.Cost.X {
		lc.refuseNode("K:Beam me up", "beam me up beside another cast route or additional cost")
	}
}
