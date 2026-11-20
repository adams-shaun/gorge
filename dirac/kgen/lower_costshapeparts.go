package kgen

// The two ability-shape guards in lowerAbility whose "beside another non-mana
// part" lists are too long to sit in a function already over the codeshape
// ceiling. They keep the exact parts the guards read (AGENTS.md: a branch in a
// long function becomes a named helper).

// waterbendOtherPart reports a cost part the Waterbend<X> shape refuses beside
// the pick.
func waterbendOtherPart(c lCost) bool {
	return c.Sac != nil || c.SacSelf || c.DiscardN > 0 || c.DiscardX || c.DiscardSelf ||
		c.Loyalty != 0 || c.SubCtr > 0 || c.PayLife > 0 || c.TapN > 0 || c.TapHost ||
		c.SacHost || c.TapPowN != 0 || c.Forage != nil || c.ExileG != nil ||
		c.Energy != 0 || c.ReturnSelf || c.ExileSelf || c.Untap || c.Return != nil ||
		c.DrawN > 0 || c.DrawBy != nil || c.DamageYouN > 0
}

// counterPlacingOtherPart reports a cost part the counter-placing shape
// refuses beside the counters.
func counterPlacingOtherPart(c lCost) bool {
	return c.Sac != nil || c.DiscardN > 0 || c.DiscardX || c.Loyalty != 0 || c.SubCtr > 0 ||
		c.SubAmongN > 0 || c.PayLife > 0 || c.TapN > 0 || c.TapPowN > 0 ||
		c.Forage != nil || c.ExileG != nil || c.Energy > 0 || c.Return != nil ||
		c.DrawN > 0 || c.DrawBy != nil || c.DamageYouN > 0 || c.Waterbend > 0 || c.WaterbendX ||
		c.BlightN > 0 || c.EvidenceN > 0
}
