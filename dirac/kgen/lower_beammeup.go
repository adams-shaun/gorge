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

// beamGrant lowers a static grant of beam me up with no cost of its own (gorge
// beamMeUpInstances: the card's mana cost). The kernel knows Scotty, Miracle Worker's
// shape only: its controller's instants and sorceries, in their main phase. The return
// goes on the granter's CardDef.FbCost.
func (lc *lowerCtx) beamGrant(sp *params, st *lStatic, aff string) error {
	if aff != "Instant.YouOwn,Sorcery.YouOwn" || !sp.is("AffectedZone", "Graveyard") ||
		!sp.is("Condition", "PlayerTurn") || !sp.is("Phases", "Main1,Main2") {
		return refuse("beam me up grant shape")
	}
	sp.consume("AddKeyword", "AffectedZone", "Condition", "Phases")
	c, err := lc.cost("Return<1/Creature>")
	if err != nil {
		return err
	}
	c.Mana = manaCost{None: true}
	lc.out.FbCost = &c
	st.Kind = "BeamGrant"
	return nil
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
