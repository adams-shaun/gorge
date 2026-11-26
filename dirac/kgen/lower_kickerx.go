package kgen

import "strings"

// K:Kicker:XMin<N> X (Thieving Skydiver): the kicker announces an {X} that
// cannot be 0. gorge folds the kicker cost into the kicked cast's own with
// cost.Plus, which keeps the larger XMin, and starts the X ask and prices the
// kicked offer at that floor (xmin_cost_test.go). The kernel keeps the {X} in
// CardDef.Kicker and the floor in CardDef.KickerCost.XMin (Flags 0, so
// kickerCostPick/kickerCostOffer stay inert); dirac/cost_sacx.go reads it.

// kickerXMin splits "XMin<N> X" into the kicker mana and the XMin-only part;
// false for any other kicker parameter.
func (lc *lowerCtx) kickerXMin(param string) (manaCost, lCost, bool) {
	head, rest, found := strings.Cut(param, " ")
	if !found || !strings.HasPrefix(head, "XMin") || strings.Contains(rest, ":") {
		return manaCost{}, lCost{}, false
	}
	var floor lCost
	if xminToken(&floor, head) != nil {
		return manaCost{}, lCost{}, false
	}
	mc, err := parseManaCost(rest)
	if err != nil || !mc.X || mc.XX || mc.None || mc.Generic != 0 || mc.Pips != [5]int{} ||
		len(mc.Hybrid)+len(mc.Twobrid)+len(mc.Phyrexian) != 0 || mc.C != 0 || mc.Snow != 0 {
		return manaCost{}, lCost{}, false
	}
	floor.Mana = manaCost{None: true}
	return mc, floor, true
}
