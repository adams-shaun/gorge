package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lifeLostExtremeValue lowers the Highest|LowestLifeLostThisTurn property of
// the PlayerCountRegisteredOpponents$ and PlayerCountDefinedRegistered[.Other]$
// groups to ExOppExtreme's OppExtLifeLost figure (gorge playerCountExtreme's
// LifeLostThisTurn arm, effects/count_player.go: the extreme over the group of
// the player's life lost this turn). The group is the living opponents in seat
// order (opponentGroup; the Registered spelling reads the same set), or with
// allPlayers every living player, the controller included
// (PlayerCountDefinedRegistered$). ok = false: not this property.
func lifeLostExtremeValue(rest string, allPlayers bool) (lValue, bool) {
	n := kop.OppExtLifeLost
	switch rest {
	case "HighestLifeLostThisTurn":
	case "LowestLifeLostThisTurn":
		if allPlayers {
			// no carrier, and the lowest over every player is the
			// controller's own zero unless all lost life: the atomdiff
			// seeds never read it true
			return lValue{}, false
		}
		n |= kop.OppExtLowest
	default:
		return lValue{}, false
	}
	if allPlayers {
		n |= kop.OppExtAllPl
	}
	return lValue{Kind: vOppExtreme, N: n}, true
}

// registeredOpponentsValue lowers PlayerCountRegisteredOpponents$<rest>.
func registeredOpponentsValue(rest string) (lValue, error) {
	switch {
	case rest == "HasPropertyLostLifeThisTurn":
		return lValue{Kind: vOppsLostLife}, nil
	case rest == oppsDealtCombatDamageBody:
		return lValue{Kind: vOppsLostLife, N: 2}, nil
	case rest == oppsDealtDamageBody:
		return lValue{Kind: vOppsLostLife, N: 3}, nil // value_turntallies.go
	}
	if hv, ok := oppsHitByLegendaryValue(rest); ok {
		return hv, nil // value_turntallies.go
	}
	if v, ok := lifeLostExtremeValue(rest, false); ok {
		return v, nil
	}
	return lValue{}, fmt.Errorf("PlayerCountRegisteredOpponents$%s", rest)
}

// definedRegisteredValue lowers PlayerCountDefinedRegistered$<rest> (every
// living player) and PlayerCountDefinedRegistered.Other$<rest> (the living
// opponents): only the life-lost extremes read.
func definedRegisteredValue(head, rest string) (lValue, error) {
	if v, ok := lifeLostExtremeValue(rest, !strings.HasSuffix(head, ".Other")); ok {
		return v, nil
	}
	return lValue{}, fmt.Errorf("%s$%s", head, rest)
}
