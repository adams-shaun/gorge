package kgen

import "strings"

// notPresentAsPresent rewrites ConditionNotPresent$ X into the
// ConditionPresent$ X | ConditionCompare$ EQ0 pair the present branch reads
// (gorge effects/conditions.go conditionNotPresentMet: met when no object of
// the group matches). It copies ps.m before writing (the map may be the
// card's own SVar params) and refuses NotPresent beside Present/Compare,
// which gorge leaves unresolved.
func (lc *lowerCtx) notPresentAsPresent(ps *params) error {
	np, ok := ps.m["ConditionNotPresent"]
	if !ok {
		return nil
	}
	if _, has := ps.m["ConditionPresent"]; has {
		return refuse("ConditionNotPresent$ beside ConditionPresent$/ConditionCompare$")
	}
	if _, has := ps.m["ConditionCompare"]; has {
		return refuse("ConditionNotPresent$ beside ConditionPresent$/ConditionCompare$")
	}
	m := make(map[string]string, len(ps.m)+1)
	for k, v := range ps.m {
		m[k] = v
	}
	delete(m, "ConditionNotPresent")
	m["ConditionPresent"] = strings.TrimSpace(np)
	m["ConditionCompare"] = "EQ0"
	ps.m = m
	return nil
}
