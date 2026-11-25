package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// definedPlayerUnionAPIs are the player-set APIs a `Defined$ A & B` join
// lowers by repeating the node once per part: the effect acts on each named
// player in script order with no state shared between the parts (gorge
// knownDefinedTargets joins the members in script order, actingPlayers then
// loops the players), so two single-player nodes are the same sequence.
// Value is the amount parameter that must be a plain literal (or absent), so
// the repeat reads the same amount twice.
var definedPlayerUnionAPIs = map[string]string{
	"Draw":     "NumCards",
	"LoseLife": "LifeAmount",
	"GainLife": "LifeAmount",
}

// definedPlayerUnion lowers Draw/LoseLife/GainLife whose Defined$ is
// `<A> & <B>` over two single players (Karazikar's "the attacking player and
// you", Cait's "the defending player and you"), repeating the node once per
// part: gorge knownDefinedTargets joins the members in script order, acting
// players then loops the players, so two single-player nodes are the same
// sequence. The pair must be You and a player the kernel binds at queue
// time that cannot be You: the defending player of an attack trigger, or the
// attacking player of an AttackersDeclared one (gorge does not name a dedupe
// for a join, so a pair that could coincide is left to the ordinary arms).
// Any other shape declines (handled=false, no refusal): the API dispatch --
// including the specialised Draw arm for `You & TriggeredTarget` (Xyris,
// Black Widow, Intel Expert), reached only when nothing earlier claims the
// node -- and defined's catch-all then decide, so this arm never shadows a
// lowerer and never replaces an ordinary refusal with its own. Every decline
// is a bare false, so the param scan's map range cannot reach an outcome.
func (lc *lowerCtx) definedPlayerUnion(sa *cards.SA, ps *params) (handled bool, err error) {
	amtKey, ok := definedPlayerUnionAPIs[sa.API]
	d, has := ps.m["Defined"]
	if !ok || !has || !strings.Contains(d, " & ") {
		return false, nil
	}
	parts := strings.Split(d, " & ")
	if len(parts) != 2 {
		return false, nil
	}
	for k := range ps.m {
		if strings.HasPrefix(k, "Condition") || strings.HasPrefix(k, "Optional") || strings.HasPrefix(k, "Unless") ||
			strings.HasPrefix(k, "Remember") || k == "Upto" || k == "Mandatory" {
			return false, nil
		}
	}
	if n, ok := ps.m[amtKey]; ok {
		for _, r := range strings.TrimSpace(n) {
			if r < '0' || r > '9' {
				return false, nil
			}
		}
	}
	sawYou, sawOther := false, false
	for i, p := range parts {
		p = strings.TrimSpace(p)
		parts[i] = p
		r, err := lc.defined(p)
		if err != nil {
			return false, nil
		}
		switch {
		case r.Kind == rYou && !sawYou:
			sawYou = true
		case r.Kind == rDefending && !sawOther, r.Kind == rTriggeredPlayer && !sawOther && lc.attackersDeclaredTrigger():
			sawOther = true
		default:
			return false, nil
		}
	}
	if !sawYou || !sawOther {
		return false, nil
	}
	for _, p := range parts {
		ps.m["Defined"] = p
		if err := lc.node(sa, ps); err != nil {
			return true, err
		}
	}
	return true, nil
}

// attackersDeclaredTrigger reports an AttackersDeclared trigger over an
// opponent's attack (AttackingPlayer$ Player.Opponent), whose trigger player
// is the declaring player and so never You.
func (lc *lowerCtx) attackersDeclaredTrigger() bool {
	if lc.ab == nil {
		return false
	}
	tr := lc.ab.Trigger
	return tr != nil && tr.Event == "AttackersDeclared" && tr.Whose == whoOpp
}

// playerRefDefined is the player-reference tail of defined: the chosen-player
// readings (lower_definedheads.go, lower_chooseplayerenter.go) and
// EnchantedController. ok is false when v is none of them.
func (lc *lowerCtx) playerRefDefined(v string) (r lRef, ok bool, err error) {
	if r, ok := lc.chosenPlayerDefined(v); ok {
		return r, true, nil
	}
	if r, ok := lc.chosenPlayerEnterDefined(v); ok {
		return r, true, nil
	}
	if v != "EnchantedController" {
		return lRef{}, false, nil
	}
	// gorge definedSpecEnchantedController: the controller of the permanent
	// the source is attached to, read live at resolution; nobody once the
	// source has left (kop.RefEnchantedCtrl).
	// Only a printed battlefield trigger (the upkeep-of-the-enchanted-
	// permanent's-controller family): build_ab gates the matching ValidPlayer$
	// to that shape.
	if lc.ab != nil && lc.ab.Kind == akTriggered && !lc.ab.Latent && lc.ab.Trigger != nil && lc.ab.Trigger.Zone == zBattlefield {
		return lRef{Kind: rEnchantedCtrl}, true, nil
	}
	return lRef{}, true, refuse("Defined$ %q outside a printed battlefield trigger", v)
}
