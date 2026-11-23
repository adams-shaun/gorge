package kgen

// pumpTimedObject routes a battlefield object's Pump Duration$ that ends on a
// game boundary: UntilYourNextTurn (pumpObjectDuration) and UntilEndOfCombat
// (pumpEndCombatObject). done is false for any other duration or subject.
func (lc *lowerCtx) pumpTimedObject(ps *params, obj lRef, zone int, d string, na, nd bool, kw string, hasKW bool) (done bool, err error) {
	if zone != zBattlefield || !lc.pumpObjectRef(obj) {
		return false, nil
	}
	switch d {
	case "UntilYourNextTurn":
		return true, lc.pumpObjectDuration(ps, obj, na, nd, kw, hasKW, d)
	case "UntilEndOfCombat":
		return true, lc.pumpEndCombatObject(ps, obj, na, nd, kw, hasKW)
	}
	return false, nil
}

// pumpEndCombatObject lowers a battlefield object's Pump Duration$
// UntilEndOfCombat (Murk Dwellers' +2/+0, Flummoxed Cyclops' and Forgestoker
// Dragon's "can't block"): one override record the kernel drops on the way
// out of the end-of-combat step (dirac/static_ovendcombat.go ovEndCombat), so
// it applies from creation and an extra combat does not revive it (gorge
// expireUntilEndOfCombat, CR 511.3). The record is applied Permanent: the
// class rides kop.Override.UntilEndCombat, not a duration constant. It mirrors
// pumpObjectDuration: a keyword part via protKwInto, a constant NumAtt$/NumDef$
// via the PTDelta record, a computed amount refuses.
func (lc *lowerCtx) pumpEndCombatObject(ps *params, obj lRef, na, nd bool, kw string, hasKW bool) error {
	cantAtk := false
	if hasKW {
		kw, cantAtk = splitKWSentence(kw, cantAttackKW)
		hasKW = kw != "" || cantAtk
	}
	if !na && !nd && !hasKW {
		return refuse("Pump Duration %q without NumAtt$/NumDef$/KW$", "UntilEndOfCombat")
	}
	o := &lOverride{CantAttack: cantAtk, UntilEndCombat: true}
	if hasKW {
		if err := lc.protKwInto(o, kw); err != nil {
			return err
		}
	}
	if na || nd {
		var err error
		att, def := cst(0), cst(0)
		if v, ok := ps.get("NumAtt"); ok {
			if att, err = lc.value(v); err != nil {
				return err
			}
		}
		if v, ok := ps.get("NumDef"); ok {
			if def, err = lc.value(v); err != nil {
				return err
			}
		}
		if !att.isConst() || !def.isConst() {
			return refuse("Pump Duration UntilEndOfCombat with a computed P/T pump")
		}
		o.PTDelta, o.PVal, o.TVal = true, att, def
	}
	lc.emitOverride(obj, o, "Permanent")
	return nil
}
