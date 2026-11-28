package kgen

import "strings"

// warpPayCost lowers K:Warp:<cost> whose cost carries a non-mana part (CR
// 702.185a; gorge keywordAltCost parses the parameter as a whole cost). The
// one shape read is mana plus a fixed PayLife<N> (Timeline Culler's
// "B PayLife<2>"): the mana is CardDef.WarpCost and the life CardDef.WarpPay,
// which the kernel's warp cast pays through its pick stage in place of the
// spell's additional cost (dirac/kw_warp.go). An {X} or hybrid mana part, an
// announced PayLife<X> and every other cost token refuse.
func (lc *lowerCtx) warpPayCost(node, param string) {
	c, err := lc.cost(param)
	if err != nil {
		lc.refuseNode(node, "warp cost %q: %v", param, err)
		return
	}
	mana := c.Mana
	if mana.None {
		mana = manaCost{}
	}
	if strings.Contains(param, ":") || mana.X || mana.XX || mana.XXX || len(mana.Hybrid) != 0 || len(mana.Twobrid) != 0 {
		lc.refuseNode(node, "warp cost %q", param)
		return
	}
	rest := c
	rest.Mana = manaCost{}
	n := rest.PayLife
	rest.PayLife = 0
	if n < 1 || n > 255 || !rest.pureMana() {
		lc.refuseNode(node, "warp cost %q: only mana plus a fixed PayLife<N> is read", param)
		return
	}
	c.Mana = manaCost{None: true}
	lc.out.Warp = &mana
	lc.out.WarpPay = &c
}

// warpGraveStatic lowers a card's own `Mode$ Continuous | Affected$ Card.Self
// | MayPlay$ True | ValidSA$ Spell.Warp | AffectedZone$ Graveyard |
// EffectZone$ Graveyard` static (Timeline Culler). gorge's generic MayPlay
// reader fails closed on a ValidSA$ other than Spell (mayPlayParamValidSA), so
// the static opens no plain cast; warpGraveyardAllowed reads it as the warp
// cast from the graveyard, for a card that prints Warp. It lowers to nothing
// but CardDef.WarpGrave. Any other shape returns false and keeps the named
// ValidSA refusal; a card printing no Warp refuses here.
func (lc *lowerCtx) warpGraveStatic(sp *params, st *lStatic, aff string) (bool, error) {
	m := func(k string) string { return strings.TrimSpace(sp.m[k]) }
	if m("MayPlay") != "True" || m("ValidSA") != "Spell.Warp" || aff != "Card.Self" ||
		m("AffectedZone") != "Graveyard" || m("EffectZone") != "Graveyard" || st.Cond.Kind != cNone {
		return false, nil
	}
	printed := false
	for _, kw := range lc.face.Keywords {
		if h, _, _ := strings.Cut(kw, ":"); h == "Warp" {
			printed = true
		}
	}
	if !printed {
		return true, refuse("Spell.Warp graveyard permission on a card with no warp cost")
	}
	sp.consume("MayPlay", "ValidSA", "AffectedZone", "EffectZone")
	lc.out.WarpGrave = true
	st.Kind = "" // the card's WarpGrave carries it: lower to nothing
	return true, nil
}
