package kgen

import (
	"strings"
)

// the values that read this turn's records.

// attackedTurnValue lowers Count$CreaturesAttackedThisTurn <spec> (gorge
// evalCov3HeadCreaturesAttackedThisTurn, engine/effects/count_cov3.go): the
// objects declared as attackers this turn that match the spec where each now
// is, each once (kernel ExAttackedTurn, an object-width arena mark). The
// spec defaults to Creature, as gorge.
func (lc *lowerCtx) attackedTurnValue(spec string) (lValue, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = "Creature"
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vAttackedTurn, F: f}, nil
}

// oppsHitByLegendaryValue lowers PlayerCountRegisteredOpponents$
// HasPropertywasDealtCombatDamageThisTurnBy <spec> GE1 (gorge
// playerCountDefinedRegistered, count_player.go): the opponents dealt combat
// damage this turn by a source matching the spec. The kernel keeps one bit per
// player for exactly Creature.Legendary at a threshold of one (any other
// spec or threshold refuses by name).
func oppsHitByLegendaryValue(rest string) (lValue, bool) {
	const exact = "HasPropertywasDealtCombatDamageThisTurnBy Creature.Legendary GE1"
	if rest == exact {
		return lValue{Kind: vOppsLostLife, N: 1}, true
	}
	return lValue{}, false
}

// oppsDealtCombatDamageBody is the bare head PlayerCountRegisteredOpponents$
// HasPropertywasDealtCombatDamageThisTurn (Tymna the Weaver): the opponents
// dealt combat damage this turn by any source, the same hit ledger as the
// legendary arm with no source condition (vOppsLostLife N 2).
const oppsDealtCombatDamageBody = "HasPropertywasDealtCombatDamageThisTurn"

// oppsDealtDamageBody is PlayerCountOpponents$ (or
// PlayerCountRegisteredOpponents$) HasPropertywasDealtDamageThisTurn (Gnoll
// War Band): the living opponents dealt any damage this turn, combat or not
// (gorge hasPropertyStateBacked, Host.DamageTakenThisTurn > 0 over
// opponentGroup; both heads read the same living-opponent group). Kernel
// vOppsLostLife N 3 (dirac/value_oppsdamaged.go).
const oppsDealtDamageBody = "HasPropertywasDealtDamageThisTurn"

// triggerCostXShape reports whether a printed trigger's own PayLife<X> cost
// carries an SVar X body gorge folds at the cost window and the kernel reads
// at the same moment (evalTriggerCostFixedX): the entering card's power on an
// enters trigger (Madame Null), the opponents dealt combat damage this turn on
// a Phase trigger at the postcombat main (Tymna the Weaver).
func triggerCostXShape(tr *lTrigger, x string) bool {
	switch {
	case tr.Event == "ETB":
		return x == "TriggeredCard$CardPower"
	case tr.Event == "Phase" && tr.Step == "Main2":
		return x == "PlayerCountRegisteredOpponents$"+oppsDealtCombatDamageBody
	}
	return false
}
