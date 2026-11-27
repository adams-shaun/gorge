package kgen

// metalcraftDynPair reports Urza's Workshop's shape: a plain ungated
// {T}: Add {C} beside a Metalcraft$-gated {T}: Add {C} for each permanent
// matching a filter tapping cannot change. gorge's census lists the source
// as one unit with both alternatives while the gate holds and the plain {C}
// alone while it does not; the kernel's evaluated pair (mana_dynpair.go)
// carries per-count views and dynPairView reads the gate on each call, so
// the gate-off view is the plain ability. Only {C} is accepted (the coloured
// evaluated pair is dynamicLargerSameColour).
func metalcraftDynPair(first, a *lAbility) bool {
	if !metalcraftGate(a.ActCond) || first.Mana == nil || a.Mana == nil {
		return false
	}
	c := *a
	c.ActCond = lCond{} // plainTapMana refuses a gate; the gate is the shape, not the mana
	if !plainTapMana(&c, false) || c.Mana.Restrict != nil || c.Mana.RestrictAb != nil {
		return false
	}
	if !plainTapMana(first, false) || !unrestrictedOne(first) || first.Mana.Colors != "C" || first.Mana.Choice || first.Mana.Each {
		return false
	}
	m, f := c.Mana, c.Mana.Amount
	if m.Colors != "C" || m.Choice || m.Each || f.Kind != vCount || f.F == nil {
		return false
	}
	return f.Add == 0 && f.Mul == 0 && f.Div == 0 && !f.Neg && len(f.Sub) == 0 && untappedStableFilter(f.F)
}
