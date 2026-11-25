package kgen

import "strings"

// notPresentAsPresent rewrites ConditionNotPresent$ X into the
// ConditionPresent$ X | ConditionCompare$ EQ0 pair the present branch reads
// (gorge effects/conditions.go conditionNotPresentMet: met when no object of
// the group matches). It copies ps.m before writing (the map may be the
// card's own SVar params) and refuses NotPresent beside Present/Compare,
// which gorge leaves unresolved.
func (lc *lowerCtx) notPresentAsPresent(ps *params) error {
	if selfNotKicked(ps) {
		return nil
	}
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

// selfNotKicked rewrites ConditionDefined$ Self | ConditionPresent$
// Card.!kicked (Desolation Giant's and Desolation Angel's "if it wasn't
// kicked" branch, Sphinx of Lost Truths' discard) into ConditionPresent$
// Card.kicked | ConditionCompare$ EQ0. The group is the one source object
// and Card matches any card, so "the source matches !kicked" is exactly "no
// member matches kicked" (gorge matchPredicate: a '!' word is the negation
// of the positive evaluation). It copies ps.m before writing.
func selfNotKicked(ps *params) bool {
	if strings.TrimSpace(ps.m["ConditionDefined"]) != "Self" || strings.TrimSpace(ps.m["ConditionPresent"]) != "Card.!kicked" {
		return false
	}
	if _, has := ps.m["ConditionCompare"]; has {
		return false
	}
	m := make(map[string]string, len(ps.m)+1)
	for k, v := range ps.m {
		m[k] = v
	}
	m["ConditionPresent"] = "Card.kicked"
	m["ConditionCompare"] = "EQ0"
	ps.m = m
	return true
}
