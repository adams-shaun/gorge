package kgen

import (
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// preventToSelfKeys are the keys a bodyless self prevention line may carry
// (preventToSelf); any other key is a different shape.
var preventToSelfKeys = map[string]bool{
	"Event": true, "Prevent": true, "ValidTarget": true, "ValidSource": true,
	"PlayerTurn": true, "IsCombat": true, "ActiveZones": true,
}

// preventToSelfSource is the ValidSource$ spelling preventToSelf admits once
// inZoneBattlefield is stripped: a type or a type with one property word
// (Creature, Artifact, Desert, Artifact.Creature). A relational source spec
// (blockedBySource, EnchantedBy, withFirst Strike) is a different read.
var preventToSelfSource = regexp.MustCompile(`^[A-Z][a-z]+(\.[A-Z][a-z]+)?$`)

// preventToSelf lowers a printed bodyless DamageDone prevention on damage to
// the permanent itself: R:Event$ DamageDone | Prevent$ True | ValidTarget$
// Card.Self or Creature.Self, with an optional ValidSource$ type filter
// (Istvan, Butcher of Eln: Creature.inZoneBattlefield), PlayerTurn$ True
// (Guardian Naga: during your turn) and IsCombat$ True. gorge reads no
// ActiveZones$ as active from anywhere (rules/replacement_match.go
// activeZonesGateOK), and damage is only ever dealt to the permanent on the
// battlefield, so an absent ActiveZones$ is the battlefield one. gorge's
// damageReplacementMatches reads ValidSource$ against the source,
// ValidTarget$ against the damaged permanent and PlayerTurn$ as the
// replacement's controller being the active player
// (replacementConditionHoldsWith). It lowers to ReplPreventDamageToSelf.
// false: not this shape.
func (lc *lowerCtx) preventToSelf(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	vt := rp.peek("ValidTarget")
	if w != nil || rp.peek("Prevent") != "True" || vt != "Card.Self" && vt != "Creature.Self" {
		return false, nil
	}
	if az, ok := rp.m["ActiveZones"]; ok && strings.TrimSpace(az) != "Battlefield" {
		return false, nil
	}
	for _, k := range sortedParamKeys(rp.m) {
		if !preventToSelfKeys[k] && !ignoredParams[k] {
			return false, nil
		}
	}
	rp.consume("Prevent", "ValidTarget", "ActiveZones")
	bits := 0
	if vt == "Creature.Self" {
		bits |= kop.ReplPrevToCreature
	}
	if v, ok := rp.get("PlayerTurn"); ok {
		if strings.TrimSpace(v) != "True" {
			return true, refuse("repl-hard: DamageDone self prevention PlayerTurn %q", v)
		}
		bits |= kop.ReplPrevToYourTurn
	}
	if v, ok := rp.get("IsCombat"); ok {
		if strings.TrimSpace(v) != "True" {
			return true, refuse("repl-hard: DamageDone self prevention IsCombat %q", v)
		}
		bits |= kop.ReplPrevToCombat
	}
	if vs, ok := rp.get("ValidSource"); ok {
		in, err := srcInPlay(vs)
		if err != nil {
			return true, err
		}
		bare := strings.NewReplacer("+inZoneBattlefield", "", ".inZoneBattlefield", "").Replace(strings.TrimSpace(vs))
		if !preventToSelfSource.MatchString(bare) {
			return true, refuse("repl-hard: DamageDone self prevention ValidSource %q", vs)
		}
		if rl.F, err = lc.filter(vs); err != nil {
			return true, err
		}
		if rl.F.Players != 0 || len(rl.F.Clauses) == 0 {
			return true, refuse("repl-hard: DamageDone self prevention ValidSource %q", vs)
		}
		if in {
			bits |= kop.ReplPrevToSrcInPlay
		}
	}
	rl.Kind, rl.Bits = "PreventDamageToSelf", bits
	return true, nil
}
