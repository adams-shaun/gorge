package kgen

import "strings"

// The two 2026-11-20 respelled referents (gorge engine/effects
// trigger_referents.go controlReferentPlayers' DelayTriggerRemembered arm and
// engine/effects/player_filter.go's OpponentOf TriggeredAttackingPlayer /
// Defending qualifiers).

// delayTriggerRememberedWord is the filter word Kitt Kanto, Mayhem Diva's
// pump target spells ("target creature that player controls").
const delayTriggerRememberedWord = "ControlledBy DelayTriggerRemembered"

// reflexTriggeredPlayer reads an ImmediateTrigger's RememberObjects$
// TriggeredPlayer inside a Phase trigger's body and reports that the body's
// DelayTriggerRemembered is the trigger's player. A Phase trigger's triggered
// player is the player whose phase it is, the active player, which the
// kernel already reads as the ActivePlayerCtrl filter word, so no new op is
// needed: delayRememberedCtrl rewrites the word to it while the body lowers.
// Any other RememberObjects$ spelling is left unread (the caller's census
// refuses it); TriggeredPlayer in another trigger mode refuses.
func (lc *lowerCtx) reflexTriggeredPlayer(ps *params, n lValue) (bool, error) {
	v, ok := ps.m["RememberObjects"]
	if !ok || strings.TrimSpace(v) != "TriggeredPlayer" {
		return false, nil
	}
	if lc.ab == nil || lc.ab.Trigger == nil || lc.ab.Latent || lc.ab.Trigger.Event != "Phase" {
		return false, refuse("ImmediateTrigger RememberObjects$ TriggeredPlayer outside a Phase trigger")
	}
	if !n.isConst() || n.N != 1 {
		return false, refuse("ImmediateTrigger RememberObjects$ TriggeredPlayer with TriggerAmount$ %s", n)
	}
	if _, ok := ps.m["RememberEach"]; ok {
		return false, refuse("ImmediateTrigger RememberObjects$ TriggeredPlayer with RememberEach$")
	}
	ps.consume("RememberObjects")
	return true, nil
}

// delayRememberedCtrl rewrites "ControlledBy DelayTriggerRemembered" to the
// ActivePlayerCtrl word inside an ImmediateTrigger body that captured
// RememberObjects$ TriggeredPlayer of a Phase trigger (reflexTriggeredPlayer).
// Anywhere else the word is left for the parser to refuse as an unknown
// property: no other capture is modelled.
func (lc *lowerCtx) delayRememberedCtrl(expr string) string {
	if !lc.reflexPhasePlayer || !strings.Contains(expr, delayTriggerRememberedWord) {
		return expr
	}
	return strings.ReplaceAll(expr, delayTriggerRememberedWord, "ActivePlayerCtrl")
}

// attackerUnattackedOppSpec is Suppressor Skyguard's CheckDefinedPlayer$:
// "that player has another opponent who isn't being attacked", the opponents
// of the attacking player that are not defending against its attack.
const attackerUnattackedOppSpec = "Player.OpponentOf TriggeredAttackingPlayer+!Defending"

// attackerUnattackedOppCheck lowers attackerUnattackedOppSpec on an
// AttackersDeclared trigger that fires for attacks on its controller
// (AttackedTarget$ You: the Defending seat is the controller). The kernel
// seats two players, so the attacker's one opponent is the attacked seat and
// no other opponent exists: the intervening-if is never true, and the
// trigger lowers with a condition that never holds. The reading is exact
// only at two seats, so the card is marked two-seat (a larger pool refuses it
// rather than reading the wrong table; gorge's own 3-seat arm is proven in
// engine/rules). ok is false for any other spelling.
func (lc *lowerCtx) attackerUnattackedOppCheck(cdp string, tr *lTrigger) (lCond, bool, error) {
	if strings.TrimSpace(cdp) != attackerUnattackedOppSpec {
		return lCond{}, false, nil
	}
	if tr.Event != "AttackersDeclared" || tr.DefPlayer != 3 {
		return lCond{}, false, refuse("CheckDefinedPlayer %q outside an AttackedTarget$ You AttackersDeclared trigger", cdp)
	}
	lc.twoSeat("CheckDefinedPlayer OpponentOf TriggeredAttackingPlayer+!Defending: the attacker's one opponent is the defender")
	return lCond{Kind: cValue, A: cst(0), Op: opGE, B: cst(1)}, true, nil
}

// checkDefinedPlayerCond lowers a trigger's CheckDefinedPlayer$ (gorge
// trigger_condition.go checkDefinedPlayerHolds): the monarch spellings (CR 725,
// lower_monarch.go), the crime one (CR 700.13) and Suppressor Skyguard's
// attacker-opponent gate are read; every other one refuses.
func (lc *lowerCtx) checkDefinedPlayerCond(cdp string, tr *lTrigger) (lCond, error) {
	if c, ok := monarchCheck(cdp); ok {
		return c, nil
	}
	if c, ok, err := lc.attackerUnattackedOppCheck(cdp, tr); err != nil || ok {
		return c, err
	}
	if cdp != "You.committedCrimeThisTurn" {
		return lCond{}, refuse("CheckDefinedPlayer %q", cdp)
	}
	return lCond{Kind: cValue, A: lValue{Kind: vCommittedCrime, Sub: []lValue{cst(1), cst(0)}}, Op: opGE, B: cst(1)}, nil
}
