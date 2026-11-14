package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for damage, fights, prevention and regeneration (lower_api.go).
func init() {
	apiPS("EachDamage", (*lowerCtx).eachDamage)
	apiPS("DealDamage", (*lowerCtx).dealDamage)
	apiPS("DamageAll", (*lowerCtx).damageAll)
	apiPS("Fight", (*lowerCtx).fight)
	apiPS("PreventDamage", (*lowerCtx).preventNext) // lower_preventnext.go
	apiOp("DamageResolve", (*lowerCtx).apiDamageResolve)
	apiPS("Regenerate", (*lowerCtx).regenerate)
	apiOp("Fog", (*lowerCtx).apiFog)
}

func (lc *lowerCtx) apiDamageResolve(sa *cards.SA, ps *params) error {
	lc.emit(lInstr{Op: "ResolveDamageMap", Note: "deal the damage map's queued damage at once"})
	return nil
}

func (lc *lowerCtx) apiFog(sa *cards.SA, ps *params) error {
	// gorge effFog: a turn-long restriction the combat damage passes
	// read (fogActive); Duration$ refuses
	if _, ok := ps.get("Duration"); ok {
		return refuse("Fog Duration$")
	}
	lc.emit(lInstr{Op: "Fog"})
	return nil
}
