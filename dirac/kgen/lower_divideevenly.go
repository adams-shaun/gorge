package kgen

import "strings"

// divideEvenly lowers DealDamage DivideEvenly$ RoundedDown (Fireball: "X
// damage divided evenly, rounded down, among any number of targets"; gorge
// effects divideEvenlyDown, Forge's DamageDealEffect). No division is chosen:
// at resolution each target still legal takes NumDmg$ over their count,
// rounded down. It is the OpDealDamageDivided node with the DmgDivideEvenly
// flag, which deals one damage batch over the slot's legal targets. Only a
// plain damage node over a target slot, with no other rider, lowers.
func (lc *lowerCtx) divideEvenly(ps *params, de string, in *lInstr) error {
	if strings.TrimSpace(de) != "RoundedDown" {
		return refuse("DivideEvenly$ %q", de)
	}
	if in.Op != "DealDamage" || in.Obj.Kind != rTarget || in.CS != "" {
		return refuse("DivideEvenly$ on a %s over %s", in.Op, in.Obj)
	}
	for _, k := range []string{"DividedAsYouChoose", "DamageMap", "ExcessSVar", "ReplaceDyingDefined", "RememberDamaged", "OptionalDecider", "DamageSource", "NoPrevention"} {
		if _, ok := ps.m[k]; ok {
			return refuse("DivideEvenly$ with %s$", k)
		}
	}
	in.Op, in.CS = "DealDamageDivided", "DivideEvenly"
	in.Note = "damage divided evenly, rounded down, among the legal targets"
	return nil
}
