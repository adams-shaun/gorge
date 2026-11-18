package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lure offer filter of askBlockers (CR 509.1c, "All creatures able to
// block CARDNAME do so."): the pair predicate no legal declaration sits
// outside, computed once per defender's declare-blockers ask. The rules are
// package combat's (block_lure.go); these two methods only bind them to the
// engine's pair oracle and blocker capacity.

// lureExtraBlocker reports whether any creature the defender controls may be
// declared against two attackers (a CanBlockAmount$/CanBlockAny$ static,
// combat.BlockCapacity). Such a declaration's maximum runs through the
// required-team search over the whole option list, so the plain-lure offer
// filter does not apply beside it.
func (e *Engine) lureExtraBlocker(defender state.PlayerID) bool {
	for _, id := range e.G.Zone(state.ZBattlefield, defender) {
		if combat.BlockCapacity(asBoard(e), id) >= 2 {
			return true
		}
	}
	return false
}

// lurePairFilter derives askBlockers' lure offer predicate for one defender:
// which (blocker, attacker) pairs some legal declaration still contains. The
// lure duty binds a creature that may declare a block against a lure
// attacker; the pair admissibility below is the offer's own oracle without
// the per-pair charge (CR 509.1c: a block needing a cost payment is not
// required, so the duty reads what CAN be declared, not what is affordable).
func (e *Engine) lurePairFilter(scope *blockPairScope, defender state.PlayerID, lure map[state.ObjID]bool) func(bid, aid state.ObjID) bool {
	zone := e.G.Zone(state.ZBattlefield, defender)
	admissible := func(b, a state.ObjID) bool {
		return !scope.minImpossible[a] && combat.CanBlock(asBoard(e), b, a)
	}
	var lureIDs []state.ObjID
	for _, aid := range scope.attackers {
		if lure[aid] {
			lureIDs = append(lureIDs, aid)
		}
	}
	duty := combat.LureDutyBlockers(zone, lureIDs, admissible)
	dutySet := make(map[state.ObjID]bool, len(duty))
	for _, b := range duty {
		dutySet[b] = true
	}
	// CR 509.1b's Max$ 1 cap: the lure attacker that hosts one blocker at
	// most. The other lure attackers host any number, as blockPairScopeFor
	// publishes the bound.
	spill := combat.LureSpillBlockers(duty, lureIDs, func(a state.ObjID) bool {
		b, ok := scope.bounds[a]
		return ok && b[1] == 1
	}, admissible)
	return func(bid, aid state.ObjID) bool {
		if lure[aid] || spill[bid] {
			return true
		}
		if dutySet[bid] {
			return false // the duty binds: the lure takes every blocker it can host
		}
		bound, ok := scope.bounds[aid]
		if !ok || bound[0] < 2 {
			return true // no blocker floor above one
		}
		declared := 0
		if o := e.G.Obj(aid); o != nil {
			declared = len(o.BlockedBy)
		}
		return combat.LureFloorMet(bound[0], declared, bid, aid, zone, dutySet, admissible)
	}
}
