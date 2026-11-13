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
	if ok, err := lc.joinForcesStore(ps, name, typ, expr); ok || err != nil {
		return err
	}
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

// joinForcesStore lowers the join-forces accumulator (Collective Voyage, Minds
// Aglow, Alliance of Arms, Shared Trauma, Mana-Charged Dragon): inside a
// RepeatPlayers$ Player loop, "StoreSVar SVar$ acc | Type$ CountSVar |
// Expression$ acc/Plus.Y" under UnlessCost$ Y (Y = Count$ChosenNumber), the
// unless wrapper (lc.unless) having the repeated player pay what they chose.
// It adds the paid amount to the resolution's total (kop.ExtJoinForces); the
// chain's trailing "Type$ Number | Expression$ 0" write resets it. The total
// is read back only by the same top-level node's chain (lc.value). Any other
// shape reports ok=false for the caller's refusal, or an error when it is
// the accumulator's own variant that does not fit.
func (lc *lowerCtx) joinForcesStore(ps *params, name, typ, expr string) (bool, error) {
	switch {
	case typ == "CountSVar" && expr == name+"/Plus.Y":
		sv := lc.face.SVars
		if strings.TrimSpace(sv[name]) != "Number$0" || strings.TrimSpace(sv["Y"]) != "Count$ChosenNumber" ||
			ps.str("UnlessCost") != "Y" || ps.str("UnlessPayer") != "Player.IsRemembered" || !ps.is("UnlessSwitched", "True") {
			return false, nil
		}
		if lc.repeatPl == 0 || !lc.repeatAllPl {
			return true, refuse("api StoreSVar: join-forces accumulator outside a RepeatEach over every player")
		}
		if lc.joinForcesName != "" && lc.joinForcesName != name {
			return true, refuse("api StoreSVar: a second join-forces accumulator %q in one chain", name)
		}
		y, err := lc.value("Y")
		if err != nil {
			return true, err
		}
		lc.joinForcesName, lc.joinForcesReset = name, false
		if lc.joinForcesNames == nil {
			lc.joinForcesNames = map[string]bool{}
		}
		lc.joinForcesNames[name] = true
		ps.consume("SVar", "Type", "Expression")
		lc.emit(lInstr{Op: "ExtJoinForces", B: y})
		return true, nil
	case typ == "Number" && expr == "0" && name == lc.joinForcesName && lc.repeatPl == 0 &&
		ps.str("ConditionCheckSVar") == "" && ps.str("UnlessCost") == "":
		lc.joinForcesReset = true
		ps.consume("SVar", "Type", "Expression")
		lc.emit(lInstr{Op: "ExtJoinForces", B: cst(0)})
		return true, nil
	}
	return false, nil
}

// pumpRememberImprinted lowers a Pump that only rebinds the remembered set to
// the previously imprinted card (Tainted Pact's DBRepeat): no characteristic
// changes, so anything else left unread refuses.
func (lc *lowerCtx) pumpRememberImprinted(ps *params) error {
	if left := ps.unread(); len(left) > 0 {
		return refuse("Pump RememberObjects$ %q beside %v", "Imprinted", left)
	}
	lc.emit(lInstr{Op: "RememberImprinted"})
	return nil
}
