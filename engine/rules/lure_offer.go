package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/combat"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lure offer filter of askBlockers (CR 509.1c, "All creatures able to
// block CARDNAME do so."): the pair predicate no legal declaration sits
// outside, computed once per defender's declare-blockers ask. The rules are
// package combat's (block_lure.go); these two functions only bind them to the
// engine's pair oracle and blocker capacity.

// lureExtraBlocker reports whether any creature the defender controls may be
// declared against two attackers (a CanBlockAmount$/CanBlockAny$ static,
// combat.BlockCapacity). Such a declaration's maximum runs through the
// required-team search over the whole option list, so the plain-lure offer
// filter does not apply beside it.
func lureExtraBlocker(b combat.Board, defender state.PlayerID) bool {
	for _, id := range b.Game().Zone(state.ZBattlefield, defender) {
		if combat.BlockCapacity(b, id) >= 2 {
			return true
		}
	}
	return false
}

// lureOfferFilter is askBlockers' offer predicate for one defender: every
// pair is offered unless a plain lure is in play (no other block duty
// a must-be-blocked attacker, a MustBlock duty, no creature with a second block) and the pair sits outside every
// legal declaration.
func lureOfferFilter(b combat.Board, scope *blockPairScope, defender state.PlayerID, lure map[state.ObjID]bool, otherDuty bool) func(bid, aid state.ObjID) bool {
	if len(lure) == 0 || otherDuty || lureExtraBlocker(b, defender) {
		return func(bid, aid state.ObjID) bool { return true }
	}
	return lurePairFilter(b, scope, defender, lure)
}

// lurePairFilter derives askBlockers' lure offer predicate for one defender:
// which (blocker, attacker) pairs some legal declaration still contains. The
// lure duty binds a creature that may declare a block against a lure
// attacker; the pair admissibility below is the offer's own oracle without
// the per-pair charge (CR 509.1c: a block needing a cost payment is not
// required, so the duty reads what CAN be declared, not what is affordable).
func lurePairFilter(b combat.Board, scope *blockPairScope, defender state.PlayerID, lure map[state.ObjID]bool) func(bid, aid state.ObjID) bool {
	g := b.Game()
	zone := g.Zone(state.ZBattlefield, defender)
	admissible := func(blk, a state.ObjID) bool {
		return !scope.minImpossible[a] && combat.CanBlock(b, blk, a)
	}
	var lureIDs []state.ObjID
	for _, aid := range scope.attackers {
		if lure[aid] {
			lureIDs = append(lureIDs, aid)
		}
	}
	// The duty reads the pairs the lure actually binds: a filtered lure
	// (Marble Priest's Walls) does not bind the other creatures.
	bound := func(blk, a state.ObjID) bool { return admissible(blk, a) && lureBinds(b, blk, a) }
	duty := combat.LureDutyBlockers(zone, lureIDs, bound)
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
	}, bound)
	return func(bid, aid state.ObjID) bool {
		if lure[aid] || spill[bid] {
			return true
		}
		if dutySet[bid] {
			return false // the duty binds: the lure takes every blocker it can host
		}
		floor, ok := scope.bounds[aid]
		if !ok || floor[0] < 2 {
			return true // no blocker floor above one
		}
		declared := 0
		if o := g.Obj(aid); o != nil {
			declared = len(o.BlockedBy)
		}
		return combat.LureFloorMet(floor[0], declared, bid, aid, zone, dutySet, admissible)
	}
}

// lureBinds reports whether the lure duty on attacker binds blocker: every
// creature able to block for the Lure sentence, only the creatures matching
// the filter for "MustBeBlockedByAll:<filter>" (CR 509.1c; Marble Priest's
// Walls, Talruum Piper's flyers). The filter resolves against the blocker
// with the attacker as the spec source.
func lureBinds(b combat.Board, blocker, attacker state.ObjID) bool {
	spec := combat.DerivedHiddenFlags(b, attacker).AllMustBlockSpec
	if spec == "" {
		return true
	}
	a := b.Game().Obj(attacker)
	return a != nil && b.MatchesSpec(spec, blocker, attacker, a.Controller, nil, nil, false)
}

// mustBeBlockedBinds reports whether blocker obeys attacker's "must be blocked
// if able" requirement: any blocker for the unfiltered form, only a creature
// matching spec for "MustBeBlockedBy <filter>" (CR 509.1c; The Masamune).
func mustBeBlockedBinds(b combat.Board, spec string, blocker, attacker state.ObjID) bool {
	if spec == "" {
		return true
	}
	a := b.Game().Obj(attacker)
	return a != nil && b.MatchesSpec(spec, blocker, attacker, a.Controller, nil, nil, false)
}
