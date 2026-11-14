package kgen

import "strings"

// presentSecondGroup ANDs Forge's second presence group (ConditionPresent2$
// with an optional ConditionCompare2$) onto the primary gate c: gorge's
// conditionMet requires both legs, each read over the battlefield as the
// ability's controller sees it (Banishing Slash's "if you control an artifact
// and an enchantment"). Only a second group beside a plain primary
// ConditionPresent$ is lifted; the second leg carries no ConditionDefined$ or
// ConditionZone$ of its own (gorge passes neither), so the primary's Defined$
// and Zone$ are not allowed to colour a reading of it either.
func (lc *lowerCtx) presentSecondGroup(ps *params, c lCond) (lCond, error) {
	cp2, ok := ps.get("ConditionPresent2")
	if !ok {
		if _, has := ps.m["ConditionCompare2"]; has {
			return lCond{}, refuse("ConditionCompare2$ without ConditionPresent2$")
		}
		return c, nil
	}
	if _, has := ps.m["ConditionDefined"]; has {
		return lCond{}, refuse("ConditionPresent2$ beside ConditionDefined$")
	}
	if _, has := ps.m["ConditionZone"]; has {
		return lCond{}, refuse("ConditionPresent2$ beside ConditionZone$")
	}
	f, err := lc.filter(cp2)
	if err != nil {
		return lCond{}, err
	}
	c2 := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	if cmp, ok := ps.get("ConditionCompare2"); ok {
		op, v, err := lc.cmpSpec(strings.TrimSpace(cmp))
		if err != nil {
			return lCond{}, err
		}
		c2.Op, c2.A = op, v
	}
	return lCond{Kind: cAnd, Sub: []lCond{c, c2}}, nil
}
