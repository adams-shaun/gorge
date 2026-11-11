package kgen

// discardHandManaShape reports whether a mana ability's cost is Lion's Eye
// Diamond's: discard the whole hand and sacrifice the source, nothing else,
// activated only as an instant (InstantSpeed$ True). gorge never lists such an
// ability in a payment window or its planner census (pay.InstantSpeedOnly);
// only a potential play's prefix search activates it at priority, so the
// kernel models it as that search's gap alone (dirac mana_discardhand.go). A
// {T} beside it (Diamond Lion) or an ability without the instant-only limit
// is refused.
func discardHandManaShape(c lCost, ps *params) bool {
	if !c.DiscardHand || !c.SacSelf || !ps.is("InstantSpeed", "True") {
		return false
	}
	c.DiscardHand, c.SacSelf = false, false
	return c.pureMana() && c.Mana.Generic == 0 && !c.Mana.X && !c.Mana.XX && c.Mana.Pips == [5]int{} &&
		c.Mana.C == 0 && len(c.Mana.Hybrid) == 0 && len(c.Mana.Twobrid) == 0 && len(c.Mana.Phyrexian) == 0
}
