package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kop.Cost.SacHalf: "sacrifice half the lands you control, rounded up"
// (Tectonic Split's own additional cost; CR 118.3, 601.2b). gorge folds the
// count into a fixed Sac<N/Land> where the cost is composed
// (engine/rules/raise_cost_extra.go resolveRaiseCostText evaluates SVar X,
// Count$Valid Land.YouCtrl/HalfUp, counting every matching permanent whether
// or not it is tapped or can be sacrificed), so nothing is announced: the part
// is a CostSac with Cost.N 0 that sacXCost does not read as the announced X,
// and its count is read from the board wherever the part's N would be. The
// pick stage poses that many picks, one per permanent as for any Sac<N>; a
// count above the candidates a CantSacrifice static leaves aborts the cast as
// a short fixed sacrifice does (sacShort).
//
// DebugSacHalfRoundsDown is the mutation switch: the count rounds down.
var DebugSacHalfRoundsDown bool

// sacOwed is the number of permanents cost c's CostSac part sacrifices for
// payer p (src: the ability's source slot, NoSlot for a spell).
func (s *State) sacOwed(k *prep, p uint8, c *kop.Cost, src Slot) int {
	if !c.SacHalf {
		return int(c.N)
	}
	n := s.countPermMatch(k, p, c.Filter, src, false)
	if DebugSacHalfRoundsDown {
		return n / 2
	}
	return (n + 1) / 2
}

// sacHalfLowers is gorgeLowersRider's arm for a spell whose own additional
// cost is the half-sacrifice. A RaiseCost's non-mana extra is never in the
// cast planner's witness (paymentPlanCastShapeDetailAltAdd: PlanCostDetail of
// the composed extra is "shape:additional_cost"), so the potential planner
// prices the composition only on its census, the printed sources and the
// pool, with no prefix search for a granted source beside it (Tectonic Split's
// own "three mana of any one color" on the lands it sacrifices): the same
// reading as the exile / forage / return parts' arm.
func (s *State) sacHalfLowers(k *prep, p uint8, c CardID, o Obj, mayPlay, anyColor bool) bool {
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	cost = s.mayRiderCost(cost, anyColor)
	return s.ungrantedPaysNoDH(k, p, c, o, &cost) ||
		mayPlay && s.mayPlayPrefixPays(k, p, c, o, anyColor)
}
