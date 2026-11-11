package kgen

import "strings"

// lower_storesvar.go: the two constant StoreSVar$ writes Tainted Pact's chain
// makes to its own RepeatCheckSVar$. gorge lets a StoreSVar$ shadow the
// printed SVar body, and Tainted Pact stores 1 when the library is empty so
// the loop stops. The repeat gate is lowered from the printed body
// (Count$ValidHand Card.IsImprinted); the empty-library write is folded into
// the gate as a "library is not empty" condition by repeat
// (repeatStoredGuard), so both the write and the post-loop reset lower to
// nothing. Any other StoreSVar$ stays refused by name.

// storeSVar lowers the constant write (or reset) of a Repeat's gate SVar.
func (lc *lowerCtx) storeSVar(ps *params) error {
	name := strings.TrimSpace(ps.str("SVar"))
	typ := strings.TrimSpace(ps.str("Type"))
	expr := strings.TrimSpace(ps.str("Expression"))
	cond := strings.TrimSpace(ps.str("ConditionCheckSVar"))
	cmp := strings.TrimSpace(ps.str("ConditionSVarCompare"))
	if lc.repeatCheckSVar == "" || name != lc.repeatCheckSVar || lc.ab != lc.repeatCheckAb || typ != "Number" {
		return refuse("api StoreSVar")
	}
	switch {
	case expr == "1" && cond == "X" && cmp == "EQ0" && strings.TrimSpace(lc.face.SVars["X"]) == "Count$ValidLibrary Card.YouOwn":
		// the library-empty stop: repeat folds it into the gate
		lc.repeatStoredGuard = true
	case expr == "0" && cond == "" && cmp == "":
		// the post-loop reset; the kernel's set lives for one resolution
	default:
		return refuse("api StoreSVar")
	}
	ps.consume("SVar", "Type", "Expression", "ConditionCheckSVar", "ConditionSVarCompare")
	return nil
}
