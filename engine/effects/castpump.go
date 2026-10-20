package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// CastPumpBonus is the literal, non-negative power/toughness bonus of the
// card's primary Pump ability, and (0, 0) for every shape the cast policy
// does not read: a curse (a negative bonus), a mixed-sign pump, an {X} or
// SVar amount, a non-creature target, an opponent-only target scope, or any
// other spell ability. It is the card fact the A6 pump timing rule reads
// (botpolicy.Card.PumpPower/PumpToughness), so a reverted projection cannot
// silently make an instant pump look like a no-op.
//
// The target scope mirrors CastRemovalTargets' comma/dot split: an
// alternative's base token is the word before the first '.', so
// "Creature.YouCtrl" still names a creature. A scope that is restricted but
// can still name one of the caster's own combatants -- "Creature.attacking",
// "Creature.blocking", "Creature.!attacking", "Creature.DefendingPlayerCtrl"
// -- still reports the literal bonus; CastPumpOwnCombatants tells the timing
// rule that the filter cannot be aimed at BOTH the caster's attackers and
// blockers, so the rule holds rather than aiming at a creature the card
// cannot legally target. The amounts are the compiled NumAtt$/NumDef$ text,
// accepted only when absent or a plain integer (strconv.Atoi accepts the
// Forge "+3" spelling); a present but dynamic value is an unknown, never a
// zero pump.
func CastPumpBonus(sa *cards.SA) (power, toughness int32) {
	f, ok := readPump(sa)
	if !ok || f.opponentOnly {
		return 0, 0
	}
	return f.power, f.toughness
}

// CastPumpOwnCombatants reports whether the card's Pump filter can target
// both the caster's own attacking and blocking creatures. It is true for an
// unrestricted creature filter and for the own-control/team-control
// qualifiers, and false for any qualifier that narrows the target set (an
// attacking-only or blocking-only pump, a non-attacker pump, the defending
// player's creatures, a colour/type/state restriction) as well as for an
// opponent-only filter. The A6 pump timing rule holds a literal pump whose
// filter is not a superset of its own combatants: aiming such a trick at a
// blocker the card cannot name would fall through to the ordinary target
// ranking and buff the opponent instead. A false answer on a card whose
// CastPumpBonus is (0, 0) means the card is not a pump the rule governs.
func CastPumpOwnCombatants(sa *cards.SA) bool {
	f, ok := readPump(sa)
	return ok && !f.opponentOnly && f.ownCombatants
}

// pumpFacts is the parsed shape of one Pump ability: the literal amounts and
// what its target scope can name.
type pumpFacts struct {
	power, toughness int32
	ownCombatants    bool
	opponentOnly     bool
}

// readPump parses the Pump ability into its facts. ok is false when the
// ability is not a literal, non-negative creature pump at all.
func readPump(sa *cards.SA) (pumpFacts, bool) {
	if sa == nil || sa.APIKind() != cards.APIPump {
		return pumpFacts{}, false
	}
	p, ok := literalPumpAmount(sa, cards.PKNumAtt)
	if !ok {
		return pumpFacts{}, false
	}
	t, ok := literalPumpAmount(sa, cards.PKNumDef)
	if !ok {
		return pumpFacts{}, false
	}
	if p < 0 || t < 0 || (p == 0 && t == 0) {
		return pumpFacts{}, false
	}
	own, opponentOnly, ok := pumpTargetScope(sa)
	if !ok {
		return pumpFacts{}, false
	}
	return pumpFacts{power: p, toughness: t, ownCombatants: own, opponentOnly: opponentOnly}, true
}

// pumpTargetScope reads ValidTgts$ alternatives whose base token is
// "creature" (case-insensitive, the same comma/dot split CastRemovalTargets
// uses). own reports whether some alternative can name every one of the
// caster's own attacking and blocking creatures; opponentOnly reports
// whether every creature alternative is restricted to creatures the caster
// does not control; ok reports whether any creature alternative exists.
func pumpTargetScope(sa *cards.SA) (own, opponentOnly, ok bool) {
	spec := TargetsOf(sa).ValidTgts
	foundCreature := false
	anyNonOpponent := false
	for alt := range strings.SplitSeq(spec, ",") {
		parts := strings.Split(strings.TrimSpace(alt), ".")
		if !strings.EqualFold(parts[0], "creature") {
			continue
		}
		foundCreature = true
		restricted, opponent := pumpQualifiers(parts[1:])
		if !opponent {
			anyNonOpponent = true
		}
		if !restricted && !opponent {
			own = true
		}
	}
	return own, foundCreature && !anyNonOpponent, foundCreature
}

// pumpQualifiers classifies one alternative's qualifiers. restricted is true
// when the filter narrows the target set below "any creature the caster
// controls"; opponent is true when it names only creatures the caster does
// not control. A "+"-joined conjunction is split like a "." qualifier, so
// "Creature.blocking+YouCtrl" is still blocking-only.
func pumpQualifiers(parts []string) (restricted, opponent bool) {
	for _, part := range parts {
		for q := range strings.SplitSeq(part, "+") {
			switch strings.ToLower(strings.TrimSpace(q)) {
			case "", "youctrl", "yourteamctrl", "other":
			case "oppctrl", "youdontctrl":
				opponent = true
				restricted = true
			default:
				restricted = true
			}
		}
	}
	return restricted, opponent
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
