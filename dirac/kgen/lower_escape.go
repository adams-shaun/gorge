package kgen

// escapeCost lowers K:Escape:<cost> (gorge escapeCost, CR 702.138a): the
// graveyard cast's mana is CardDef.AltCost and its printed exile part
// CardDef.EscCost, paid through the cast's pick stage in place of the spell's
// additional cost (dirac/kw_escape.go). The corpus shape is mana plus one
// ExileFromGrave<N/Card.Other/other> part (exiling OTHER cards from your
// graveyard). A bare keyword, an {X} mana part, a sacrifice or tap part, a
// CARDNAME self-exile and every other cost token is not read exactly.
func (lc *lowerCtx) escapeCost(node, param string) {
	if param == "" {
		lc.refuseNode(node, "escape cost %q", param)
		return
	}
	actCost := lc.actCost
	lc.actCost = true // Exile<N/Spec> is a spell's own cost part here (lower.go's token gate)
	c, err := lc.cost(param)
	lc.actCost = actCost
	if err != nil {
		lc.refuseNode(node, "escape cost %q: %v", param, err)
		return
	}
	mana := c.Mana
	if mana.X {
		lc.refuseNode(node, "escape mana part %s", mana)
		return
	}
	if mana.None {
		mana = manaCost{}
	}
	rest := c
	rest.Mana = manaCost{}
	if rest.SacX {
		lc.refuseNode(node, "escape cost part other than an exile part")
		return
	}
	if rest.Sac != nil || rest.SacN > 0 {
		// Exile<1/Land>, the printed battlefield-exile part (Lunar
		// Hatchling, the corpus's only escape card with one): a counted part
		// of its own (kop.Cost.N/Filter), and the cast carries one counted
		// part, the grave exile (kernel pick cost, build.go cost's census)
		lc.refuseNode(node, "escape cost's battlefield-exile part: the cast's pick cost is one counted part, the grave exile")
		return
	}
	if rest.ExileG == nil || rest.ExileGN == 0 || rest.ExileGSelf || rest.exileGCard || rest.ExileHand || rest.ExileGBf {
		lc.refuseNode(node, "escape cost without an ExileFromGrave<N/Card.Other> part")
		return
	}
	// the grave-exile part steps aside only for the census: pureMana is the
	// nothing-beside check, and ExileG* is the part itself
	exG, exN := rest.ExileG, rest.ExileGN
	rest.ExileG, rest.ExileGN = nil, 0
	if !rest.pureMana() {
		lc.refuseNode(node, "escape cost parts other than the printed exile part")
		return
	}
	rest.ExileG, rest.ExileGN = exG, exN
	if lc.out.SpellCost != nil {
		lc.refuseNode(node, "escape beside a spell additional cost (one pick cost per cast)")
		return
	}
	lc.out.Escape, lc.out.EscCost = &mana, &rest
}

// refuseEscapeCombos refuses the escape keyword beside any other cast route
// or additional cost: Escape, Flashback, Harmonize, Mayhem and BeamMeUp all
// claim CardDef.AltCost, one pick cost is paid per cast, and the kernel's
// escape cast is one plain alternative mana cost plus the printed exiles
// (dirac/kw_escape.go). Multi-face cards are refused: the escape walk casts
// the card's one printed face.
func (lc *lowerCtx) refuseEscapeCombos() {
	out := lc.out
	if out.Escape == nil {
		return
	}
	if out.Flashback != nil || out.Harmonize != nil || out.Mayhem != nil || out.BeamMeUp != nil || out.FbCost != nil {
		lc.refuseNode("K:Escape", "Escape beside another graveyard-cast route (all claim CardDef.AltCost)")
	}
	if out.Convoke || out.Improvise || out.Kicker != nil || out.KickerCost != nil || out.Warp != nil || out.Plot != nil ||
		out.Sneak != nil || out.Impending != nil || out.Bestow != nil || out.SpellCost != nil || out.AltAddCost != nil ||
		out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Casualty != 0 || out.Cost.X || out.Spree ||
		out.Madness != nil || out.Miracle != nil || out.Dash != nil || out.Evoke != nil || out.MayFlash != nil {
		lc.refuseNode("K:Escape", "Escape beside another cast route or additional cost")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Escape", "Escape on a multi-face card")
	}
}
