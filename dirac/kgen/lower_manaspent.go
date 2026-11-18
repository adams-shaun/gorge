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
// AddsCounters$ (Opal Palace, Biophagus, Animal Attendant, Guildmages'
// Forum) stays refused by name (refuseAddsCounters). gorge implements the
// rider -- the producing ability's grant rides the cast and rules'
// entry-counter plan puts the counters as the creature enters -- but its
// planner defers every source carrying it, as it does TriggersWhenSpent$.
//
// TriggersWhenSpent$ (Pyromancer's Goggles) stays refused: gorge's planner
// defers such a source (pay.PlanHasSpecialProductionParam), a census gap the
// kernel does not model. Any other spelling leaves its params unread, and
// finishMana refuses.
func (lc *lowerCtx) manaSpentRiders(ps *params, ab *lAbility, m *lMana) error {
	if _, ok := ps.m["AddsCounters"]; ok {
		return refuseAddsCounters()
	}
	if v, ok := ps.m["AddsNoCounter"]; ok {
		return lc.manaNoCounter(ps, ab, m, strings.TrimSpace(v))
	}
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

// refuseAddsCounters is the by-name refusal of the AddsCounters$ mana rider.
// gorge's payment planner defers a mana ability carrying it
// (pay.PlanHasSpecialProductionParam, source:special_production): its census
// never lists the source, autopay never taps it, and only a potential play's
// prefix script or the manual mana window does, so a cast the rider's mana
// would fund is not offered by autopay. The kernel models a deferred source
// for Pyromancer's Goggles' typed {R} alone (mana_deferred.go: floatDeferred,
// the gfDeferred gap in the scripted-prefix and window walks); an any-colour
// source (Biophagus, Animal Attendant) or a fee-paying commander-identity one
// beside a {T} ability (Opal Palace, Guildmages' Forum, whose counter count is
// the commander's cast count) is no such unit. Lowering the rider alone,
// with the planner's autopay unmodelled, plays a different set of casts than
// gorge (measured on the Carnelian Orb's lockstep pool: the kernel offered
// casts the reference did not).
func refuseAddsCounters() error {
	return refuse("AddsCounters$ mana rider: gorge's planner defers the source (pay.PlanHasSpecialProductionParam: autopay never taps it, only a prefix script or the manual mana window does) and the kernel's deferred-source gap models only Pyromancer's Goggles' typed {R}, not an any-colour or fee-paying source whose spent unit tags a creature spell's entry counters")
}

// manaNoCounter reads AddsNoCounter$ True beside a spell restriction (Cavern
// of Souls, Delighted Halfling; CR 106.6): the {T}, any-colour mana that may
// pay only a matching spell and makes that spell uncounterable. gorge floats
// the unit in a restriction batch tagged AddsNoCounter; a payment that carves
// it for a spell sets the spell's can't-be-countered flag (pay
// emitRestrictedManaSpend, FlagNoCounter). The kernel floats it in its own
// batch kind (kop.Ability.ManaNoCounter) and stamps the spell its payment
// spent it on (dirac mana_nocounter.go). gorge's planner defers every source
// carrying the parameter; the kernel's restricted any-colour census gap
// (Secluded Courtyard's shape) is that deferral.
//
// Boseiju's "!Permanent" (a source with no restriction and a life cost) has
// no kernel carrier: its mana floats unrestricted beside a condition on the
// spell's face, and the source is a deferred {C} source of a shape
// mana_deferred.go does not vet.
func (lc *lowerCtx) manaNoCounter(ps *params, ab *lAbility, m *lMana, v string) error {
	if v != "True" {
		return refuse("AddsNoCounter$ %s: the unit's can't-be-countered condition (a !Permanent test on the spell's face, no spend restriction) has no kernel carrier: the restricted batch records the spell it paid for, not a face test, and gorge's planner defers the source (pay.PlanHasSpecialProductionParam), a {T}+PayLife deferred {C} source the kernel's deferred model does not vet", v)
	}
	if ab.Kind != akMana || ab.Loyalty || !plainTapCost(ab.Cost) || m.Restrict == nil || m.RestrictAb != nil || m.NonSpell ||
		m.Special != "" || m.Choice || m.Colors != "WUBRG" || !m.Amount.isConst() || m.Amount.N != 1 {
		return refuse("AddsNoCounter$ True beside a mana ability that is not a plain {T} one-mana any-colour source restricted to spells")
	}
	for _, k := range []string{"AddsCounters", "TriggersWhenSpent", "PersistentMana", "AddsKeywords"} {
		if _, ok := ps.m[k]; ok {
			return refuse("AddsNoCounter$ beside another mana-spend rider (%s)", k)
		}
	}
	m.NoCounter = true
	ps.consume("AddsNoCounter")
	return nil
}
