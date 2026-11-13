package kgen

// K:Escalate:<cost> (CR 702.120a): an additional cost on a modal spell, paid
// once for each mode chosen beyond the first. gorge reads the keyword's raw
// parameter at the cast (beginCast), clamps the mode ask's Max to one plus the
// escalations the board can pay for (castModeAsk) and folds (modes - 1) copies
// of the cost into the total once the modes are chosen (applyCastModes).
//
// The kernel prices a chosen mode set like Spree's: CardDef.HasModeCost with
// every mode's own cost zero, and CardDef.Escalate the cost paid per extra
// mode (dirac/kw_escalate.go). The keyword therefore sets Spree, which keeps
// it off every cast route Spree is kept off, and modeCost reads no ModeCost$
// beside it.

// lowerEscalate reads the keyword parameter: a plain mana cost. Any other
// cost (tapXType<1/Creature>, Discard<1/Card>) or a second Escalate or a
// Spree/Tiered card is refused by name.
func (lc *lowerCtx) lowerEscalate(node, param string) {
	if lc.out.Escalate != nil || lc.out.Spree {
		lc.refuseNode(node, "Escalate beside another per-mode cost keyword")
		return
	}
	mc, err := parseManaCost(param)
	if err != nil || param == "" || mc.X || mc.None {
		lc.refuseNode(node, "escalate cost %q", param)
		return
	}
	lc.out.Escalate, lc.out.Spree = &mc, true
}
