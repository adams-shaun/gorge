package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CastPumpBonus is the literal, non-negative power/toughness bonus of the
// card's primary Pump ability, and (0, 0) for every shape the cast policy
// does not read: a curse (a negative bonus), a mixed-sign pump, an {X} or
// SVar amount, a non-creature target, or any other spell ability. It is the
// card fact the A6 pump timing rule reads (botpolicy.Card.PumpPower/
// PumpToughness), so a reverted projection cannot silently make an instant
// pump look like a no-op.
//
// The target scope mirrors CastRemovalTargets' comma/dot split: an
// alternative's base token is the word before the first '.', so
// "Creature.YouCtrl" still names a creature. The amounts are the compiled
// NumAtt$/NumDef$ text, accepted only when absent or a plain integer
// (strconv.Atoi accepts the Forge "+3" spelling); a present but dynamic
// value is an unknown, never a zero pump.
func CastPumpBonus(sa *cards.SA) (power, toughness int32) {
	if sa == nil || sa.APIKind() != cards.APIPump {
		return 0, 0
	}
	if !pumpTargetsCreature(sa) {
		return 0, 0
	}
	p, ok := literalPumpAmount(sa, cards.PKNumAtt)
	if !ok {
		return 0, 0
	}
	t, ok := literalPumpAmount(sa, cards.PKNumDef)
	if !ok {
		return 0, 0
	}
	if p < 0 || t < 0 || (p == 0 && t == 0) {
		return 0, 0
	}
	return p, t
}

// pumpTargetsCreature reports whether some ValidTgts$ alternative's base
// token is "creature" (case-insensitive).
func pumpTargetsCreature(sa *cards.SA) bool {
	spec := TargetsOf(sa).ValidTgts
	for alt := range strings.SplitSeq(spec, ",") {
		base, _, _ := strings.Cut(strings.TrimSpace(alt), ".")
		if strings.EqualFold(base, "creature") {
			return true
		}
	}
	return false
}

// literalPumpAmount reads one pump amount: absent is 0, and a present value
// is accepted only when it is a plain integer ("+3", "-2", "0").
func literalPumpAmount(sa *cards.SA, key cards.ParamKey) (int32, bool) {
	if !sa.HasParam(key) {
		return 0, true
	}
	n, err := strconv.Atoi(strings.TrimSpace(sa.ParamStr(key)))
	if err != nil {
		return 0, false
	}
	return int32(n), true
}
