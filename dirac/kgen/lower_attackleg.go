package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// attackLegEmptyKey is the dotted referent the Commander curses' second leg
// names for its beneficiary: `TriggeredAttackingPlayer.Opponent+
// controlsCreature.attacking Player.EnchantedBy` (curse_of_opulence's
// DBToken, curse_of_disturbance's DBToken, curse_of_verbosity's DBDraw,
// curse_of_vitality's DBGainLife, curse_of_bounty's DBUntap). The set is
// empty at every seat count and in every turn state: only the ACTIVE player
// ever declares attackers (CR 508.1 — the declare-attackers turn-based
// action is the active player's alone), so no opponent of the attacking
// player can control an attacking creature, and gorge's dotted selector
// (engine/effects/context.go) therefore resolves to no player. The leg is a
// provable no-op, and the lowerer skips it instead of refusing the card.
// (gorge plays it as the no-op it is — engine/effects, the same resolved
// empty set reaching each of these APIs.)
const attackLegEmptyKey = "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"

// attackLegParams are the params a skipped empty leg may carry, per API:
// every one of them scales with the (empty) player set — no owners, no
// tokens; no players, no draws, life or untaps — so dropping the node loses
// nothing. Anything else refuses: a param whose effect is not proportional
// to the player set (a Remember$ side, a Condition$) must not vanish.
var attackLegParams = map[string]map[string]bool{
	"Token":    {"TokenOwner": true, "TokenScript": true, "TokenAmount": true, "SubAbility": true},
	"Draw":     {"Defined": true, "SubAbility": true, "NumCards": true},
	"GainLife": {"Defined": true, "SubAbility": true, "LifeAmount": true},
	"UntapAll": {"Defined": true, "SubAbility": true, "ValidCards": true},
}

// skipEmptyAttackLeg consumes an AttackersDeclared leg whose beneficiary set
// is the empty one attackLegEmptyKey names. The caller's chain loop runs it
// before the node lower: done means the whole node is consumed and lowered
// to nothing.
func (lc *lowerCtx) skipEmptyAttackLeg(node *cards.SA, p *params) (bool, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "AttackersDeclared" {
		return false, nil
	}
	key := ""
	allow := attackLegParams[node.API]
	switch node.API {
	case "Token":
		key = p.str("TokenOwner")
	case "Draw", "GainLife", "UntapAll":
		key = p.str("Defined")
	default:
		return false, nil
	}
	if allow == nil || strings.TrimSpace(key) != attackLegEmptyKey {
		return false, nil
	}
	left := p.unread()
	for _, k := range left {
		if !allow[k] {
			return true, refuse("%s$ %s: skipped empty attacking-player leg with %s$", node.Kind, node.API, k)
		}
	}
	for k := range allow {
		p.consume(k)
	}
	return true, nil
}
