package kgen

import (
	"sort"
	"strings"
)

// aiHintSVars reports whether an AddSVar$ value names only SVars of this
// face in Forge's "SVar:<Name>:<Value>" grant shape: gorge's layer walk
// grants that variable to the affected object (parseSVarGrant), and its only
// reader, Engine.GrantedSVar, has no rules caller -- the corpus's granted
// variables are AI hints (HasAttackEffect, MustBeBlocked, AITapDown, SacMe).
func (lc *lowerCtx) aiHintSVars(v string) bool {
	names := strings.FieldsFunc(v, func(r rune) bool { return r == '&' || r == ',' })
	if len(names) == 0 {
		return false
	}
	for _, n := range names {
		if !strings.HasPrefix(lc.face.SVars[strings.TrimSpace(n)], "SVar:") {
			return false
		}
	}
	return true
}

// bareCondition lowers a node's bare Condition$ (gorge conditionBareMet):
// Kicked reads the resolving spell's cast provenance (the kernel's
// CondKicked reads the resolving entry, so only a spell's own chain
// lowers); Threshold and Delirium read the resolving controller's
// graveyard. Any other spelling, or one beside a ConditionPresent$/
// ConditionCheckSVar$ gate, refuses.
func (lc *lowerCtx) bareCondition(ps *params, c string) (lCond, error) {
	for _, k := range []string{"ConditionPresent", "ConditionCheckSVar", "ConditionDefined", "ConditionCompare", "ConditionSVarCompare"} {
		if _, ok := ps.m[k]; ok {
			return lCond{}, refuse("Condition$ %q beside %s$", c, k)
		}
	}
	switch c {
	case "Kicked", "Bargain":
		if c == "Bargain" && !lc.out.Bargain {
			return lCond{}, refuse("Condition$ Bargain without Bargain")
		}
		if lc.ab == nil || lc.ab.Kind != akSpell {
			return lCond{}, refuse("Condition$ %s outside a spell", c)
		}
		return lCond{Kind: cKicked}, nil
	case "OptionalCost":
		return lc.optCostCond(ps) // lower_optcost.go
	case "Threshold":
		return lCond{Kind: cThreshold}, nil
	case "Delirium":
		return deliriumCond(), nil
	}
	return lCond{}, refuse("Condition$ %q", c)
}

// playerTurnCondition lowers a lone ConditionPlayerTurn$ True|False (gorge
// conditions.go: g.Active compared with the resolving controller); beside
// any other Condition*$ gate it refuses.
func (lc *lowerCtx) playerTurnCondition(ps *params, v string) (lCond, error) {
	var beside []string
	for k := range ps.m {
		if strings.HasPrefix(k, "Condition") && k != "ConditionPlayerTurn" && k != "ConditionDescription" {
			beside = append(beside, k)
		}
	}
	if len(beside) > 0 {
		sort.Strings(beside)
		return lCond{}, refuse("ConditionPlayerTurn$ beside %s$", beside[0])
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		return lCond{Kind: cYourTurn}, nil
	case "false":
		return lCond{Kind: cYourTurn, Neg: true}, nil
	}
	return lCond{}, refuse("ConditionPlayerTurn$ %q", v)
}
