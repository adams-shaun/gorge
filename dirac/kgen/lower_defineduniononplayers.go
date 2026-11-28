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
	"Scry":     "ScryNum", // effLookAndArrange asks and arranges each library in turn
}

// definedPlayerUnionBatched are the player-set APIs gorge resolves for every
// acting player as ONE action (effDiscard holds every player's choice and
// flushes the discards in one discard and zone batch; effMill brackets every
// library in one action and zone batch), so a Mode$ DiscardedAll /
// MilledAll / ChangesZoneAll trigger fires once where two nodes fire it
// twice, and a discard replacement asks after both choices, not between
// them. A pair the repeat would otherwise lower refuses by that name.
var definedPlayerUnionBatched = map[string]string{
	"Discard": "discards",
	"Mill":    "mills",
}

// definedPlayerUnion lowers Draw/LoseLife/GainLife/Scry whose Defined$ is
// `<A> & <B>` over two single players (Karazikar's "the attacking player and
// you", Cait's "the defending player and you"), repeating the node once per
// part: gorge knownDefinedTargets joins the members in script order, acting
// players then loops the players, so two single-player nodes are the same
// sequence. The pair, in either script order, must be You and a player the
// kernel binds at queue time that cannot be You (neverYou): gorge
// de-duplicates players at first appearance (playerIDsFromTargets), where a
// repeated node would act twice, so a pair that could coincide is left to
// the ordinary arms (definedSet refuses it by name).
// Any other shape declines (handled=false, no refusal): the API dispatch --
// including the specialised Draw arm for `You & TriggeredTarget` (Xyris,
// Black Widow, Intel Expert), reached only when nothing earlier claims the
// node -- and defined's catch-all then decide, so this arm never shadows a
// lowerer and never replaces an ordinary refusal with its own, but for a
// batched API's pair (definedPlayerUnionBatched), refused by name. Every decline
// is a bare false, so the param scan's map range cannot reach an outcome.
func (lc *lowerCtx) definedPlayerUnion(sa *cards.SA, ps *params) (handled bool, err error) {
	amtKey, ok := definedPlayerUnionAPIs[sa.API]
	verb, batched := definedPlayerUnionBatched[sa.API]
	ok = ok || batched
	if batched {
		amtKey = "NumCards"
	}
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
	// parts stay in script order: the repeat acts on them in that order.
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
		case !sawOther && lc.neverYou(r):
			sawOther = true
		default:
			return false, nil
		}
	}
	if !sawYou || !sawOther {
		return false, nil
	}
	if batched {
		return true, refuse("Defined$ %q: gorge %s for both players in one batch", d, verb)
	}
	for _, p := range parts {
		ps.m["Defined"] = p
		if err := lc.node(sa, ps); err != nil {
			return true, err
		}
	}
	return true, nil
}

// neverYou reports whether r is one player the kernel binds at queue time
// who can never be the resolving controller: the defending player of the
// controller's own attack (ownAttackTrigger: an Attacks trigger of the
// controller's attacker, or an AttackersDeclared trigger of its attack, whose
// per-defender attacked player is that defending seat), or the trigger
// player of a trigger whose player filter is opponents only (an
// AttackersDeclared trigger over an opponent's attack; a Phase trigger of an
// opponent's or the enchanted opponent's step, whose trigger player is that
// step's active player).
func (lc *lowerCtx) neverYou(r lRef) bool {
	if lc.ab == nil || r.N != 0 {
		return false
	}
	switch r.Kind {
	case rDefending:
		return lc.ownAttackTrigger()
	case rTriggeredPlayer:
		tr := lc.ab.Trigger
		return lc.attackersDeclaredTrigger() ||
			tr != nil && tr.Event == "Phase" && (tr.Whose == whoOpp || tr.Whose == whoEnchantedOpp)
	}
	return false
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
	if v == "EnchantedPlayer" {
		// gorge definedSpecEnchantedPlayer: the player the source Aura
		// enchants, read live; nobody once the source has left
		// (kop.RefEnchantedPlayer). Printed battlefield triggers only (the
		// Aura's own enters trigger), as EnchantedController below.
		if lc.ab != nil && lc.ab.Kind == akTriggered && !lc.ab.Latent && lc.ab.Trigger != nil &&
			(lc.ab.Trigger.Zone == zBattlefield || lc.ab.Trigger.Zone == zNone) && lc.granted == 0 {
			return lRef{Kind: rEnchantedPlayer}, true, nil
		}
		return lRef{}, true, refuse("Defined$ %q outside a printed battlefield trigger", v)
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
