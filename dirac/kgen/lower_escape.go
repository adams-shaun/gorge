package kgen

import "strings"

// escapeCost lowers K:Escape:<cost> (gorge escapeCost, CR 702.138a): the
// graveyard cast's mana is CardDef.AltCost and its printed exile part
// CardDef.EscCost, paid through the cast's pick stage in place of the spell's
// additional cost (dirac/kw_escape.go). The corpus shape is mana plus one
// ExileFromGrave<N/Card.Other/other> part (exiling OTHER cards from your
// graveyard), optionally led by one Exile<N/Spec> battlefield part (Lunar
// Hatchling's Exile<1/Land>, CardDef.EscBfCost: a counted part of its own, so
// a second Cost value; the exile token must precede the grave one, gorge asks
// in token order). A bare keyword, an {X} mana part, a sacrifice or tap part,
// a CARDNAME self-exile and every other cost token is not read exactly.
func (lc *lowerCtx) escapeCost(node, param string) {
	if param == "" {
		lc.refuseNode(node, "escape cost %q", param)
		return
	}
	actCost := lc.actCost
	lc.actCost, lc.escCost = true, true // Exile<N/Spec> is a spell's own cost part here (lower.go's token gate)
	c, err := lc.cost(param)
	lc.actCost, lc.escCost = actCost, false
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
	var bf *lCost
	if rest.Sac != nil || rest.SacN > 0 {
		// Exile<N/Spec>, the printed battlefield-exile part (Lunar Hatchling's
		// Exile<1/Land>): its own Cost (CardDef.EscBfCost), picked before the
		// grave exile as gorge's exAsk walks the parts in token order. Only an
		// exile beside the grave exile, exile token first, is read.
		if !rest.SacExile || rest.Sac == nil || rest.SacN < 1 || rest.SacSelf || rest.SacHost || rest.HostExile {
			lc.refuseNode(node, "escape cost's sacrifice part (only an Exile<N/Spec> battlefield part is read)")
			return
		}
		ib, ig := strings.Index(param, "Exile<"), strings.Index(param, "ExileFromGrave<")
		if ig < 0 {
			lc.refuseNode(node, "escape cost without an ExileFromGrave<N/Card.Other> part")
			return
		}
		if ib < 0 || ib > ig {
			lc.refuseNode(node, "escape cost's battlefield-exile part after the ExileFromGrave part (gorge asks in token order)")
			return
		}
		bf = &lCost{Sac: rest.Sac, SacN: rest.SacN, SacExile: true}
		rest.Sac, rest.SacN, rest.SacExile = nil, 0, false
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
	lc.out.Escape, lc.out.EscCost, lc.out.EscBf = &mana, &rest, bf
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
