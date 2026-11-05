package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// deferredMana vets Pyromancer's Goggles' mana ability: "{T}: Add {R}. When
// that mana is spent to cast a red instant or sorcery spell, copy that spell
// and you may choose new targets for the copy" (TriggersWhenSpent$). gorge's
// planner defers any such source (pay.PlanHasSpecialProductionParam), so the
// kernel marks the ability kop.Ability.ManaDeferred and runs the rider as a
// SpellCast trigger of the source that reads the payment's tapped source
// (Tf2ManaFromSelf, gorge ValidSA$ Spell.ManaFromCard.StrictlySelf). Only this
// exact shape lowers: Produced$ R, a plain {T} cost, the rider SVar a SpellCast
// of a red instant or sorcery whose body is a lone CopySpellAbility of the
// triggering spell with MayChooseTarget$ True; anything else refuses.
func (lc *lowerCtx) deferredMana(ab *lAbility, ps *params, rider string) error {
	if ab.Kind != akMana || !ab.Cost.Tap || !plainTapCost(ab.Cost) || lc.face == nil {
		return refuse("TriggersWhenSpent$ on a mana ability beyond a plain {T}")
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
	for k, want := range map[string]string{"ValidCard": "Instant.Red,Sorcery.Red"} {
		if strings.TrimSpace(t.Params[k]) != want {
			return refuse("TriggersWhenSpent$ rider %s$ %q", k, t.Params[k])
		}
	}
	for k := range t.Params {
		switch k {
		case "Mode", "ValidCard", "Execute", "TriggerDescription":
		default:
			return refuse("TriggersWhenSpent$ rider param %s$", k)
		}
	}
	exec := strings.TrimSpace(t.Params["Execute"])
	sa := cards.ResolveSVar(lc.face.SVars, exec)
	if sa == nil || sa.API != "CopySpellAbility" || sa.Sub != nil || strings.TrimSpace(sa.Params["Defined"]) != "TriggeredSpellAbility" ||
		strings.TrimSpace(sa.Params["MayChooseTarget"]) != "True" {
		return refuse("TriggersWhenSpent$ rider body is not a lone CopySpellAbility of the triggering spell")
	}
	for k := range sa.Params {
		switch k {
		case "DB", "Defined", "MayChooseTarget":
		default:
			return refuse("TriggersWhenSpent$ rider body param %s$", k)
		}
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
