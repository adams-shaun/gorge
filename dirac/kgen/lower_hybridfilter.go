package kgen

// hybridFilterMana: a is "{c1/c2}, {T}: Add {c1}{c1}, {c1}{c2}, or {c2}{c2}"
// (Fetid Heath, the Shadowmoor and Eventide filter lands), the second mana
// ability beside a plain unrestricted "{T}: Add {C}". Its fee is one hybrid
// symbol of two distinct WUBRG colours beside {T}, nothing else, and it makes
// two mana of the two colours the symbol names (Produced$ Combo c1 c2 |
// Amount$ 2), unrestricted.
//
// gorge's planner census leaves it an unrelaxable gap (a hybrid fee has no
// relaxed alternative, paymentPlanRelaxedAlternatives), but its prefix search
// still funds it as a non-relaxable gap: its fee is one mana per hybrid pip
// (paymentPlanCensusOf's fund), so the search first taps a covered source to
// float the pip. Its activation ask lists the Combo as one option per colour
// of the amount ("Add WW", "Add BB": pay.ManaAbilityComboColours), after the
// {C} of the first ability. The kernel plays it as a filter whose fee is the
// hybrid pip (kop.HybridFilterManaAb, mana_filterhybrid.go).
func hybridFilterMana(first, a *lAbility) bool {
	c, m := &a.Cost, a.Mana
	if len(c.Mana.Hybrid) != 1 || c.Mana.Generic != 0 || c.Mana.Pips != [5]int{} || c.Mana.C != 0 || c.Mana.X || c.Mana.XX || c.Mana.XXX ||
		c.Mana.None || c.Mana.Snow != 0 || len(c.Mana.Twobrid) != 0 || len(c.Mana.Phyrexian) != 0 || c.Mana.XColor != 0 {
		return false
	}
	b := *a
	b.Cost.Mana = manaCost{}
	if !plainTapMana(&b, false) || !m.Choice || m.Each || m.Restrict != nil || m.RestrictAb != nil || m.NonSpell || m.Deferred || m.Split != nil ||
		m.Rider != 0 || m.Amount.Kind != vConst || m.Amount.N != 2 || len(m.Colors) != 2 || m.Colors[0] == m.Colors[1] {
		return false
	}
	h := c.Mana.Hybrid[0]
	if !(h[0] == m.Colors[0] && h[1] == m.Colors[1] || h[0] == m.Colors[1] && h[1] == m.Colors[0]) {
		return false
	}
	return plainTapMana(first, false) && unrestrictedOne(first) && first.Mana.Colors == "C" && !first.Mana.Choice
}
