package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// effectEndCombatCantAttack reports the Effect shape whose Duration$
// UntilEndOfCombat the kernel's emblem can hold: its statics are all
// "Mode$ CantAttack | ValidCard$ Creature" (Overencumbered's "creatures can't
// attack this combat"). gorge registers the restriction as a continuous effect
// and expireUntilEndOfCombat drops it as the end-of-combat step ends
// (effect_combatstatics.go, effectUntilEOT; CR 511.3), so the emblem is an
// until-end-of-turn one that also ends there (kop.Emblem.UntilEndCombat); a
// combat that never ran leaves it to the cleanup sweep. Every other spelling
// of that Duration$ keeps refusing in effectEmblemDuration.
func effectEndCombatCantAttack(ps *params, ss []cards.Static) bool {
	if !ps.is("Duration", "UntilEndOfCombat") || len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if s.Mode != "CantAttack" {
			return false
		}
	}
	return true
}
