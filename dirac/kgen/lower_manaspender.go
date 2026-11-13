package kgen

import "strings"

// lower_manaspender.go: Defined$ "You & TriggeredSpellAbility>ManaSpender.<who>"
// (Yume, Chronicler of Valor's DB$ Draw inside the assist trigger: "you and
// that player each draw a card"). Forge's set is the trigger's controller and
// the player(s) other than them who spent mana to cast the triggered spell.

// manaSpenderYouAnd lowers a player-set body whose Defined$ is
// "You & TriggeredSpellAbility>ManaSpender.Other" as the trigger's
// controller alone. The kernel's cast payments are single-payer: the caster
// pays every part of the cost, so no player other than the trigger's
// controller can ever be a mana spender of the cast, and the spender arm of
// the set is empty in every state the kernel reaches. Gorge agrees in
// decision: its ValidSA$ gate for this trigger,
// "Spell.ManaSpentBy Player.Other", is an unmodelled two-field clause and
// fails closed, so the trigger never fires there either
// (engine/rules/trigmatch/cast.go ValidSAMatches). A future multi-payer
// (assist) payment model must teach this read beside it.
//
// The every-blocker probe (probeBody) lowers a refused trigger's body on its
// own with no trigger bound; there the context guard cannot run and the shape
// lowers for the census, while the real lowering still refuses it outside a
// SpellCast trigger body. ok is false for any other Defined$ value.
func (lc *lowerCtx) manaSpenderYouAnd(ps *params) (lRef, bool, error) {
	d, ok := ps.get("Defined")
	if !ok || strings.TrimSpace(d) != "You & TriggeredSpellAbility>ManaSpender.Other" {
		return lRef{}, false, nil
	}
	if !(lc.p != nil && lc.p.allBlockers && lc.ab.Latent && lc.ab.Name == "Probe") {
		tr := lc.ab.Trigger
		if tr == nil || tr.Event != "SpellCast" || !lc.trigBody || lc.ab.Latent {
			return lRef{}, true, refuse("Defined$ %q outside a SpellCast trigger body", d)
		}
	}
	return lRef{Kind: rYou}, true, nil
}
