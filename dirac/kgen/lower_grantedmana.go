package kgen

import "strings"

// Granted mana abilities (tokmana). A Continuous AddAbility$ grant of a
// mana ability (Springleaf Parade's "Creature tokens you control have
// '{T}: Add one mana of any color'", Cryptolith Rite, Chromatic Lantern)
// reaches the kernel's cast offers through its latent-ability record
// (genPrep.grantPC: one planner view per granted mana ability), which
// prices a plain "{T}: Add <colours> x N" and nothing else. gorge's
// planner census treats every granted ability as a gap it cannot witness
// (pay.PaymentAbility excludes grants) and prices the play after a
// scripted prefix activates the granted sources (potentialPrefixScript),
// which the kernel copies for that one shape (grantPrefix / grantSearch).
// A spend restriction on spells and abilities (Clement's "spend only on
// creature spells", Ambulatory Reactor's artifact spells and abilities of
// artifact sources) rides the same search: the tapped grant floats its mana
// in a restricted batch (State.rb) spendable only as the restriction
// allows. "{T}, Pay N life" rides it too (Forgotten Monument's Caves: the
// step pays the life, CR 119.4, and is skipped when the payer's life is
// below N; dirac mana_grantcost.go), and so does "Sacrifice this" alone
// (Lotus Ring's equipped creature: the step sacrifices the source, tapped
// or summoning-sick alike, CR 602.2b, and a resolution-time payment's
// manual mana window sacrifices it too). Any other shape -- a mana
// fee or a dynamic amount (Evendo's "{G}, {T}: Add {G} for each creature"),
// {T} with a sacrifice (a granted Treasure ability) or another non-tap part,
// a chosen colour -- would be counted as plain mana or not at all, so it is
// refused.

// grantedManaShape reports why granted mana ability ab is outside the
// kernel's granted-source shape ("" when it is a plain tap ability).
func grantedManaShape(ab *lAbility) string {
	if ab.Kind != akMana {
		return ""
	}
	c := &ab.Cost
	if !grantedManaCostShape(c) || c.Untap || c.Sac != nil || c.DiscardN > 0 || c.DiscardSelf || c.Loyalty != 0 || c.SubCtr > 0 ||
		c.TapN > 0 || c.TapHost || c.SacHost || c.TapPowN > 0 || c.Forage != nil || c.ExileG != nil || c.Energy != 0 ||
		c.ReturnSelf || c.ExileSelf || c.Return != nil || c.DrawN > 0 {
		return "granted mana ability whose cost is not {T}"
	}
	m := &c.Mana
	if m.Generic != 0 || m.X || m.XX || m.C != 0 || len(m.Hybrid) != 0 || m.Pips != [5]int{} {
		return "granted mana ability with a mana fee"
	}
	if ab.Mana == nil {
		return "granted mana ability without a production"
	}
	if ab.Mana.RestrictAb != nil && ab.Mana.Restrict == nil || filterHasSpecial(ab.Mana.Restrict) || filterHasSpecial(ab.Mana.RestrictAb) {
		// an activated-only restriction has no kop form, and a Static.*
		// clause is not a spell or ability filter
		return "granted mana ability with a spend restriction"
	}
	if ab.Mana.Special == "ReflectedSelf" && ab.Mana.SpecialF == nil {
		return "" // the grantee's own colours (lower_reflectedmana.go; the shape checks above hold)
	}
	if ab.Mana.Special != "" || ab.Mana.SpecialF != nil {
		return "granted mana ability with a special production"
	}
	if ab.Mana.Amount.Kind != vNone && (!ab.Mana.Amount.isConst() || ab.Mana.Amount.N < 1) {
		return "granted mana ability with a dynamic amount"
	}
	if ab.Mana.Colors == "" || ab.Mana.Colors == "Chosen" {
		return "granted mana ability of a chosen colour"
	}
	return ""
}

// grantedManaCostShape reports whether c's tap, sacrifice and life parts are
// one of the three granted shapes the kernel's grant search steps pay: {T}
// alone, {T} + pay N life (Forgotten Monument; the step pays the life), or
// sacrifice this alone (Lotus Ring; the step sacrifices the source, tapped
// or not). The other cost parts are grantedManaShape's.
func grantedManaCostShape(c *lCost) bool {
	if c.LifeX {
		return false
	}
	switch {
	case c.Tap && !c.SacSelf:
		return c.PayLife >= 0 && c.PayLife <= 255
	case c.SacSelf && !c.Tap:
		return c.PayLife == 0
	}
	return false
}

// colorsInWUBRGOrder reports whether the colour letters of s are in W U
// B R G order (the kernel's colour mask order).
func colorsInWUBRGOrder(s string) bool {
	last := -1
	for _, r := range s {
		i := strings.IndexRune("WUBRGC", r)
		if i < 0 || i <= last {
			return false
		}
		last = i
	}
	return true
}
