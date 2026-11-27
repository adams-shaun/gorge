package kgen

// triggeredSpellTarget is Defined$ TriggeredSpellAbilityTargets (gorge
// definedSpecTriggeredSpellAbilityTargets: the cast spell's root and chain
// targets, de-duplicated, read off the spell object).
//
// Only the single-target arm lowers: a SpellCast trigger with IsSingleTarget$
// True whose TargetsValid$ names objects only (Loki, God of Lies), so the
// spell's one distinct target is an object and the set is one referent. The
// kernel captures it as the trigger is queued (kop.RefTriggerSpellTarget).
// A set of targets (Storm, Windrider; Dack Fayden's emblem) has no kernel
// reference and stays refused, as does a trigger whose target could be a
// player.
func (lc *lowerCtx) triggeredSpellTarget(v string) (lRef, error) {
	tr := lc.ab.Trigger
	if tr == nil || tr.Event != "SpellCast" {
		return lRef{}, refuse("Defined$ %q outside a SpellCast trigger", v)
	}
	if tr.F2 == nil || tr.F2.Players != 0 || len(tr.F2.Clauses) == 0 {
		return lRef{}, refuse("Defined$ %q: a target set that may include a player", v)
	}
	if tr.SingleTarget {
		return lRef{Kind: rTriggeredSpellTarget}, nil
	}
	return lRef{Kind: rTriggeredSpellTargets}, nil
}
