package kgen

// K:Offering:<Type> (CR 702.48; gorge rules/offering.go): "You may cast this
// spell any time you could cast an instant by sacrificing a <Type> and paying
// the difference in mana costs between this and the sacrificed <Type>." The
// kernel's offering cast (dirac/kw_offering.go) rides the may-flash action;
// the sacrifice is the casualty-shaped election before the targets, and the
// reduction subtracts the sacrificed permanent's mana cost symbol by symbol.

// lowerOffering reads the printed quality, one bare type word, as the
// sacrifice filter gorge's offeringSacrificePart spells
// ("Permanent.<Type>+YouCtrl").
func (lc *lowerCtx) lowerOffering(node, param string) {
	ok := param != ""
	for i := 0; i < len(param); i++ {
		if c := param[i]; (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			ok = false
		}
	}
	if !ok {
		lc.refuseNode(node, "offering quality %q", param)
		return
	}
	if lc.out.Offering != nil {
		lc.refuseNode(node, "two Offering keywords")
		return
	}
	f, err := lc.filter("Permanent." + param + "+YouCtrl")
	if err != nil {
		lc.refuseNode(node, "offering quality %q: %v", param, err)
		return
	}
	lc.out.Offering = f
}

// refuseOfferingCombos refuses Offering beside any other cast cost route or
// cost announcement (the kernel's offering cast prices one plain mana cost of
// a single-face card, its election posed before the targets where gorge's
// sacAsk poses it after convoke, X and delve), the refuseMayFlashCombos shape.
func (lc *lowerCtx) refuseOfferingCombos() {
	out := lc.out
	if out.Offering == nil {
		return
	}
	if out.Kicker != nil || out.Multikicker != nil || out.Repeated != nil || out.Warp != nil || out.Plot != nil || out.Flashback != nil || out.SpellCost != nil || out.AltAddCost != nil ||
		out.Convoke || out.Improvise || out.Delve || out.Offspring != nil || out.Gift || out.MorphCost != nil || out.Sneak != nil || out.Bestow != nil || out.Impending != nil ||
		out.Mayhem != nil || out.Harmonize != nil || out.Madness != nil || out.Casualty != 0 || out.Bargain || out.OptCost != nil || out.Teamwork != 0 || out.Spree ||
		out.WebSling != nil || out.Evoke != nil || out.Dash != nil || out.MayFlash != nil || out.Buyback != nil || out.Entwine != nil || out.BeholdN != 0 {
		lc.refuseNode("K:Offering", "Offering beside an alternative or additional cost")
	}
	if c := out.Cost; c.X || c.None || c.Snow != 0 || len(c.Hybrid) != 0 || len(c.Twobrid) != 0 || len(c.Phyrexian) != 0 {
		lc.refuseNode("K:Offering", "Offering on an {X}, hybrid or uncastable spell")
	}
	if len(lc.card.Faces) != 1 || lc.card.AlternateMode != "" {
		lc.refuseNode("K:Offering", "Offering on a multi-face card")
	}
}
