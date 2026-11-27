package kgen

// effectTriggers lowers a DB$ Effect carrying a Triggers$ line. Every shape
// it knows returns from here; the ones it does not stay refused by name.
func (lc *lowerCtx) effectTriggers(ps *params, tr string) error {
	if done, err := lc.effectImprintShieldFrom(ps); done || err != nil { // lower_effectimprintshield.go
		return err
	}
	if _, st := ps.m["StaticAbilities"]; st {
		if done, err := lc.effectUntapDenial(ps, tr); done || err != nil { // lower_untapdenial.go
			return err
		}
	}
	if _, st := ps.m["ReplacementEffects"]; st {
		if done, err := lc.effectUntapDenial(ps, tr); done || err != nil { // lower_untapdenial.go
			return err
		}
	}
	if _, st := ps.m["StaticAbilities"]; st && ps.m["RememberObjects"] != "" {
		if done, err := lc.effectPlaneCurse(ps, tr); done || err != nil { // lower_planecurse.go
			return err
		}
		return lc.effectStaticTriggerPair(ps, tr) // lower_effectcaptured.go
	}
	if done, err := lc.contractLossEffect(ps, tr); done || err != nil { // lower_contract.go
		return err
	}
	return lc.emblem(ps, tr)
}
