package kgen

import "strings"

// fightExcess reads a Fight's ExcessSVar$ (CR 120.10,
// ). It returns true for the one shape the kernel records
// exactly, and refuses every other spelling.
//
// gorge fixed the inline reflexive (the gated ImmediateTrigger that once
// resolved inline when Excess was published) in b0016ca15, so Rhino's
// Rampage's reflexive Destroy now queues like the kernel's
// OpReflexiveTrigger. gorge's effFight captures each hit's lethal before any
// hit lands for every recipient passing excessConditionHolds; with
// ExcessSVarCondition$ Creature.targetedBy+OppCtrl and ValidTgts$
// Creature.OppCtrl, only the hit dealt TO the Fight's own target binds: the
// other fighter is the parent target (Creature.YouCtrl), targetedBy too but
// not OppCtrl while it is still a legal target. A fight that did not happen
// leaves the name unbound, which reads 0.
func (lc *lowerCtx) fightExcess(ps *params) (bool, error) {
	es, ok := ps.get("ExcessSVar")
	if !ok {
		return false, nil
	}
	if ps.str("ExcessSVarCondition") != "Creature.targetedBy+OppCtrl" ||
		ps.str("ValidTgts") != "Creature.OppCtrl" || ps.str("Defined") != "ParentTarget" {
		return false, refuse("Fight ExcessSVar$ shape")
	}
	ps.consume("ExcessSVarCondition")
	lc.excess = strings.TrimSpace(es)
	return true, nil
}

// dealExcess checks the DealDamage ExcessSVar$ shapes the kernel's
// DmgTrackExcess (OpDealDamageEx) records exactly: gorge's
// excessLethal binds only a creature (toughness less marked damage) or a
// planeswalker (loyalty); a Battle or other permanent binds nothing, where
// the kernel would record the whole amount. So every clause of the target
// filter must be creature and/or planeswalker; the source's-power Bite (a
// DamageSource$) has no excess record at all.
func (lc *lowerCtx) dealExcess(obj, dmgSrc lRef) error {
	if dmgSrc.Kind != rNone {
		return refuse("ExcessSVar with a DamageSource$")
	}
	if obj.Kind != rTarget || obj.N < 0 || obj.N >= len(lc.ab.Targets) {
		return refuse("ExcessSVar on a damage recipient that is not a target")
	}
	f := lc.ab.Targets[obj.N].F
	if f == nil || len(f.Clauses) == 0 {
		return refuse("ExcessSVar on a target without a creature/planeswalker filter")
	}
	for _, c := range f.Clauses {
		if c.Types == 0 || c.Types&^(tCreature|tPlaneswalker) != 0 {
			return refuse("ExcessSVar on a target that may be neither creature nor planeswalker")
		}
	}
	return nil
}
