package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// noContractSpec is the DamageDone ValidTarget$ of "deals combat damage to a
// player who doesn't have a contract counter" (Miss Highwater, the corpus's
// one Player.counters_ spelling). gorge's player matcher reads it as the
// recipient's live player-counter count (engine/effects/player_filter.go,
// splitPlayerCountCompare over state.Player.Counter("Contract")).
const noContractSpec = "Player.counters_EQ0_Contract"

// noContract lowers noContractSpec where the trigger reads it: the damaged
// player is any player (TfToPlayer, no Filter2) and the kernel's per-seat
// Contract count gates the fire (kop.Tf5DmgNoContract,
// dirac/ctr_contract.go). Only the carrier's shape lowers: combat damage
// dealt by the battlefield source itself, the fire path (fireDamageEvGen)
// that reads the flag.
func noContract(tp *params, tr *lTrigger, t cards.Trigger) error {
	if !selfCombatDamageDone(tp, tr, t) {
		return refuse("ValidTarget$ %s off a battlefield Card.Self combat DamageDone", noContractSpec)
	}
	tr.F2 = &lFilter{Players: plBoth}
	tr.NoContract = true
	return nil
}

// selfCombatDamageDone reports whether t is a combat DamageDone line whose
// source is the battlefield card itself, with no player half read yet.
func selfCombatDamageDone(tp *params, tr *lTrigger, t cards.Trigger) bool {
	return t.Mode == "DamageDone" && tp.is("CombatDamage", "True") && tr.Zone == zNone &&
		tr.F != nil && len(tr.F.Clauses) == 1 && tr.F.Clauses[0].Flags&fSelf != 0 && tr.F.Players == 0
}

// contractLossEffect lowers Miss Highwater's "for as long as they have a
// contract counter, when they lose the game, for each artifact and creature
// they controlled, create a token that's a copy of it": a Duration$
// Permanent Effect remembering the damaged player whose Triggers$ are one
// LosesGame line on that player (ValidPlayer$ Player.IsRemembered) and,
// optionally, the Effect's Static$ Always self-exile once the player has no
// contract counter. The Effect lowers to nothing at every seat count, because
// gorge, the reference, arms nothing for it: effects.effEffect
// splits Triggers$ on whitespace only, so the comma list
// "LoseGameTrigger,ExileEffect" is one unparseable name (a Note, no delayed
// trigger), and in a pod, where a loss does not end the game, no copy of the
// loser's artifacts and creatures is ever made. The kernel matches that; when
// gorge arms the LosesGame line, the four-seat pod lockstep
// (TestFourSeatMissHighwaterPodsLockstep) diverges at the first contracted
// loss and this lowering owes the trigger. done is false for any other
// Triggers$ list, which the emblem lowering reads (and refuses) as before.
func (lc *lowerCtx) contractLossEffect(ps *params, list string) (done bool, err error) {
	names := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' })
	if len(names) == 0 || len(names) > 2 {
		return false, nil
	}
	var loses, exile int
	for _, n := range names {
		t, ok := cards.ParseTriggerLine(lc.face.SVars[n])
		switch {
		case !ok:
			return false, nil
		case t.Mode == "LosesGame" && t.Params["ValidPlayer"] == "Player.IsRemembered":
			loses++
		case t.Mode == "Always" && t.Params["Static"] == "True" && contractSelfExile(lc.face.SVars, t):
			exile++
		default:
			return false, nil
		}
	}
	if loses != 1 || exile > 1 || !ps.is("Duration", "Permanent") || ps.str("RememberObjects") != "TriggeredTarget" {
		return false, nil
	}
	ps.consume("Triggers", "Duration", "RememberObjects")
	return true, nil
}

// contractSelfExile reports whether t, an Effect's Mode$ Always line, exiles
// the Effect itself once its remembered player has no contract counter.
func contractSelfExile(svars map[string]string, t cards.Trigger) bool {
	if t.Params["CheckDefinedPlayer"] != "Player.IsRemembered+counters_EQ0_Contract" {
		return false
	}
	b := cards.ResolveSVar(svars, t.Params["Execute"])
	return b != nil && b.API == "ChangeZone" && b.Params["Origin"] == "Command" &&
		b.Params["Destination"] == "Exile" && b.Params["Defined"] == "Self" && b.Sub == nil
}

// contractCounter is the PutCounter CounterType$ of a player's contract
// counter (Miss Highwater; OpExt ExtPutContract).
const contractCounter = "Contract"

// putContract lowers PutCounter CounterType$ Contract: a player's counter,
// so only a player operand lowers (gorge effPutCounter's player arm, one
// PlayerCounterChange per player). The kernel keeps the per-seat count
// (dirac/ctr_contract.go); any other recipient refuses.
func (lc *lowerCtx) putContract(obj lRef, n lValue) error {
	switch obj.Kind {
	case rYou, rEachOpp, rEachPlayer, rTriggeredPlayer:
	default:
		return refuse("contract counters on %s", obj)
	}
	lc.emit(lInstr{Op: "PutCounter", Obj: obj, A: n, CS: contractCounter})
	return nil
}
