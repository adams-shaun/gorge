package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// atkLastTurnSpec is the DamageDone ValidTarget$ of "deals combat damage to a
// player, if that player attacked you during their last turn" (O-Kagachi,
// Vengeful Kami). gorge's player matcher reads it as Forge's
// attackedYouTheirLastTurn property (engine/effects matchesPlayerSingleSpec
// over state.Player.AttackedSeatsLastTurn): the damaged player declared an
// attack whose defending seat was the trigger's controller during the damaged
// player's most recent completed turn.
const atkLastTurnSpec = "Player.attackedYouTheirLastTurn"

// atkYouLastTurn lowers atkLastTurnSpec where the trigger reads it: the
// damaged player is any player (TfToPlayer, no Filter2) and the kernel's
// per-seat attack record gates the fire (kop.Tf5DmgAttackedYouLastTurn,
// dirac/trig_atklastturn.go). Only the corpus carrier's shape lowers: combat
// damage dealt by the battlefield source itself, the one kernel fire path
// (fireDamageEvGen) that reads the flag.
func atkYouLastTurn(tp *params, tr *lTrigger, t cards.Trigger) error {
	if t.Mode != "DamageDone" || !tp.is("CombatDamage", "True") || tr.Zone != zNone ||
		tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Clauses[0].Flags&fSelf == 0 || tr.F.Players != 0 {
		return refuse("ValidTarget$ %s off a battlefield Card.Self combat DamageDone", atkLastTurnSpec)
	}
	tr.F2 = &lFilter{Players: plBoth}
	tr.AtkYouLastTurn = true
	return nil
}
