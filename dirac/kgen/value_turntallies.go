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
