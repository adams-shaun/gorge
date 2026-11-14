package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// deferredMana vets a deferred mana ability: "{T}: Add {R}. When that mana is
// spent to cast a red instant or sorcery spell, copy that spell and you may
// choose new targets for the copy" (Pyromancer's Goggles), "Add {G}. When you
// spend this mana to cast a spell with mana value 6 or greater, draw a card"
// (Gilanra) (TriggersWhenSpent$). gorge's planner defers any such source
// (pay.PlanHasSpecialProductionParam), so the kernel marks the ability
// kop.Ability.ManaDeferred and runs the rider as a SpellCast trigger of the
// source that reads the payment's tapped source (Tf2ManaFromSelf, gorge
// ValidSA$ Spell.ManaFromCard.StrictlySelf). The mana is one fixed colour
// (lowerMana), the cost a plain {T}; the rider SVar is a SpellCast of the
// source's controller's spell (any ValidCard$ lowerTrigger reads) whose body is
// any chain lowerTriggered lowers; anything else refuses.
func (lc *lowerCtx) deferredMana(ab *lAbility, ps *params, rider string) error {
	if ab.Kind != akMana || !ab.Cost.Tap || !plainTapCost(ab.Cost) || lc.face == nil {
		return refuse("TriggersWhenSpent$ on a mana ability beyond a plain {T}")
	}
	// The kernel tracks one floating deferred source per player (genState.defSrc):
	// two sources' mana in one payment is unmodelled, so only a legendary source
	// (never two on the battlefield, CR 704.5j) lowers.
	if !lc.face.IsLegendary() {
		return refuse("TriggersWhenSpent$ on a source that is not legendary")
	}
	for k := range ps.m {
		switch k {
		case "AB", "Cost", "Produced", "TriggersWhenSpent", "SpellDescription":
		default:
			return refuse("TriggersWhenSpent$ beside mana parameter %s$", k)
		}
	}
	body := lc.face.SVars[rider]
	t, ok := cards.ParseTriggerLine(body)
	if !ok || t.Mode != "SpellCast" {
		return refuse("TriggersWhenSpent$ rider %q is no SpellCast trigger", rider)
	}
	if vp := strings.TrimSpace(t.Params["ValidActivatingPlayer"]); vp != "" && vp != "You" {
		return refuse("TriggersWhenSpent$ rider ValidActivatingPlayer$ %q", vp)
	}
	for k := range t.Params {
		switch k {
		case "Mode", "ValidCard", "ValidActivatingPlayer", "Execute", "TriggerDescription":
		default:
			return refuse("TriggersWhenSpent$ rider param %s$", k)
		}
	}
	sa := cards.ResolveSVar(lc.face.SVars, strings.TrimSpace(t.Params["Execute"]))
	if sa == nil {
		return refuse("TriggersWhenSpent$ rider has no Execute$ body")
	}
	t.Effect = sa
	p := make(map[string]string, len(t.Params)+1)
	for k, v := range t.Params {
		p[k] = v
	}
	p["ValidSA"] = "Spell.ManaFromCard.StrictlySelf"
	t.Params = p
	lc.deferTrig = &t
	ps.consume("TriggersWhenSpent")
	return nil
}
