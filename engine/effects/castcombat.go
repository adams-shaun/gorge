package effects

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CastCombatTrick reports whether the card's primary spell ability is a
// combat trick: a pump, mass pump, protection grant, or +1/+1 counter put
// that can target or affect a creature the caster controls.
func CastCombatTrick(sa *cards.SA) bool {
	if sa == nil {
		return false
	}
	switch sa.APIKind() {
	case cards.APIPump:
		power, ok := literalPumpAmount(sa, cards.PKNumAtt)
		if !ok {
			return false
		}
		toughness, ok := literalPumpAmount(sa, cards.PKNumDef)
		if !ok || power < 0 || toughness < 0 {
			return false
		}
		pump := PumpOf(sa)
		if power == 0 && toughness == 0 && len(pump.Grant.KW) == 0 {
			return false
		}
		if permanent, _ := durationTiming(pump.Grant.Duration); permanent {
			return false
		}
		_, opponentOnly, ok := pumpTargetScope(sa)
		return ok && !opponentOnly
	case cards.APIPumpAll:
		power, ok := literalPumpAmount(sa, cards.PKNumAtt)
		if !ok {
			return false
		}
		toughness, ok := literalPumpAmount(sa, cards.PKNumDef)
		if !ok || power < 0 || toughness < 0 {
			return false
		}
		pump := PumpOf(sa)
		if power == 0 && toughness == 0 && len(pump.Grant.KW) == 0 {
			return false
		}
		if permanent, _ := durationTiming(pump.Grant.Duration); permanent {
			return false
		}
		if pump.PumpZone != "" && (!pump.PumpZoneOK || !slices.Contains(pump.PumpZones, state.ZBattlefield)) {
			return false
		}
		return pumpAllCanAffectOwnCreature(sa)
	case cards.APIProtection:
		if strings.TrimSpace(sa.ParamStr(cards.PKGains)) == "" {
			return false
		}
		return targetCanNameOwnCreature(TargetsOf(sa).ValidTgts)
	case cards.APIPutCounter:
		put := PutCounterOf(sa)
		if !slices.Contains(put.Kinds, "P1P1") {
			return false
		}
		if put.CounterNum.Present {
			n, ok := literalInt(put.CounterNum)
			if !ok || n <= 0 {
				return false
			}
		}
		return targetCanNameOwnCreature(TargetsOf(sa).ValidTgts)
	}
	return false
}

func pumpAllCanAffectOwnCreature(sa *cards.SA) bool {
	spec := strings.TrimSpace(sa.ParamStr(cards.PKValidCards))
	if spec == "" {
		return true
	}
	return specCanNameNonOpponentCreature(spec)
}

func targetCanNameOwnCreature(spec string) bool {
	if strings.TrimSpace(spec) == "" {
		return false
	}
	return specCanNameNonOpponentCreature(spec)
}

func specCanNameNonOpponentCreature(spec string) bool {
	for alt := range strings.SplitSeq(spec, ",") {
		parts := strings.Split(strings.TrimSpace(alt), ".")
		if !strings.EqualFold(parts[0], "creature") {
			continue
		}
		_, opponent := pumpQualifiers(parts[1:])
		if !opponent {
			return true
		}
	}
	return false
}
