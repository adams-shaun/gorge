package kgen

import "strings"

// manaSpentRiders reads the mana-spend rider the kernel carries on an AB$
// Mana (CR 106.6): AddsKeywords$ Haste | AddsKeywordsValid$
// Spell.Dragon+Creature | AddsKeywordsUntil$ UntilEOT (Carnelian Orb of
// Dragonkind). gorge floats such mana in a source-bearing batch
// (engine/effects/mana_effect.go provenanceOnly) that a payment carves before
// the plain units of its colour, captures the grant when the spell matches
// (pay.emitRestrictedManaSpend) and applies it as the permanent enters
// (rules/altcast.go applyManaKeywordGrant). Its planner plans the source as
// an ordinary one (AddsKeywords* are known mana params, pay/planshape.go).
// The kernel taps a source for a cast inside that cast's payment and spends
// the pool at once, so the rider's mana is spent exactly when the payment
// tapped the source (kop.Ability.ManaHasteValid, dirac/mana_spentrider.go).
//
// TriggersWhenSpent$ (Pyromancer's Goggles) stays refused: gorge's planner
// defers such a source (pay.PlanHasSpecialProductionParam), a census gap the
// kernel does not model. Any other spelling leaves its params unread, and
// finishMana refuses.
func (lc *lowerCtx) manaSpentRiders(ps *params, ab *lAbility, m *lMana) error {
	plain := ab.Kind == akMana && !ab.Loyalty && plainTapCost(ab.Cost) && m.Restrict == nil && m.RestrictAb == nil &&
		!m.NonSpell && m.Special == "" && !m.Choice && len(m.Colors) == 1 && m.Amount.isConst() && m.Amount.N == 1
	if !plain {
		return nil
	}
	for _, k := range []string{"AddsCounters", "TriggersWhenSpent", "AddsNoCounter", "PersistentMana"} {
		if _, ok := ps.m[k]; ok {
			return nil // one rider per ability
		}
	}
	if strings.TrimSpace(ps.m["AddsKeywords"]) != "Haste" || strings.TrimSpace(ps.m["AddsKeywordsValid"]) != "Spell.Dragon+Creature" ||
		strings.TrimSpace(ps.m["AddsKeywordsUntil"]) != "UntilEOT" {
		return nil
	}
	f, err := lc.filter("Card.Dragon+Creature")
	if err != nil {
		return err
	}
	m.HasteValid = f
	ps.consume("AddsKeywords", "AddsKeywordsValid", "AddsKeywordsUntil")
	return nil
}
