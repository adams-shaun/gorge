package kgen

import "strings"

// the ConditionZone$ node gate and the Delirium$ / Revolt$
// replacement gates. Each lowers to the condition kinds already in the
// table (presentZoneCond's count, deliriumCond, the Revolt value), so no new
// enum ordinal is added.
//
// nodeConditionZone applies a node's ConditionPresent$ ConditionZone$
// (gorge effects/conditions.go: ConditionZone narrows the presence count to
// that zone; effects/conditions.go conditionMetZone counts every object in
// every player's zone whose face matches the spec, and Card.YouOwn narrows
// the owner). An empty zone or Battlefield keeps the battlefield read.
// ConditionDefined$ selects gorge's defined-group path, a different read, so
// it refuses. presentZoneCond (lower_remembered.go) refuses Hand, Library, Stack
// and Command itself.
func (lc *lowerCtx) nodeConditionZone(ps *params, c lCond, spec string) (lCond, error) {
	zone, ok := ps.get("ConditionZone")
	if !ok {
		return c, nil
	}
	if d := strings.TrimSpace(ps.m["ConditionDefined"]); d == "TriggeredCard" && strings.TrimSpace(zone) == "Stack" && spec == "Card.inZoneStack" {
		return c, nil // lowerCond rewrote the pair into the filter (lower_effect.go)
	}
	if d, ok := ps.m["ConditionDefined"]; ok && strings.TrimSpace(d) != "" {
		return lCond{}, refuse("ConditionPresent$ with ConditionZone$ %q and ConditionDefined$ %q", zone, d)
	}
	switch strings.TrimSpace(zone) {
	case "", "Battlefield":
		return c, nil
	}
	return presentZoneCond(c, zone, spec, lc)
}

// replCondKeys applies the Delirium$ and Revolt$ gates a replacement line
// carries (gorge rules/replacement_match.go replacementConditionHoldsWith:
// both are independent AND gates on the Moved, DamageDone and every other
// replacement family). Delirium$ True is four or more card types in the
// controller's graveyard (CR 207.2c); Revolt$ True is a permanent that left
// the battlefield under the controller's control this turn (CR 207.2c). Any
// other value refuses, and so does a key beside another condition.
//
// Only the replacement kinds whose kernel path actually evaluates Repl.Cond
// may carry them: the enters program (turn_gen.go, copy_asenters.go) and the
// damage doubler (repl_multipliers.go dblDmgMatch). Every other family
// reads the condition as its own different gate (replPlus' granted
// max-speed) or not at all, so lowering the key there would silently drop
// it. OthersEnterWith is excluded: its walk never reads Repl.Cond.
func (lc *lowerCtx) replCondKeys(rp *params, rl *lRepl, kind string) error {
	del, hasDel := rp.get("Delirium")
	rev, hasRev := rp.get("Revolt")
	if !hasDel && !hasRev {
		return nil
	}
	switch kind {
	case "EntersWith", "DoubleDamageToOpps":
	default:
		return refuse("Delirium$/Revolt$ on a %s replacement (the kernel does not read Repl.Cond there)", kind)
	}
	if hasDel {
		if strings.TrimSpace(del) != "True" {
			return refuse("Delirium$ %q", del)
		}
		if rl.Cond.Kind != cNone {
			return refuse("two replacement conditions")
		}
		rl.Cond = deliriumCond()
	}
	if hasRev {
		if strings.TrimSpace(rev) != "True" {
			return refuse("Revolt$ %q", rev)
		}
		if rl.Cond.Kind != cNone {
			return refuse("two replacement conditions")
		}
		rl.Cond = lCond{Kind: cValue, A: lValue{Kind: vRevolt}, Op: opGE, B: cst(1)}
	}
	return nil
}
