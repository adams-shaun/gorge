package kgen

// a printed graveyard activated ability's "activate only
// if" (Activation$ Delirium / Threshold, IsPresent$ with PresentZone$, a
// PlayerTurn$ gate). gorge gates every zone's offer with the same
// activationConditionOK / abilityPresentHolds on the ACTIVATOR, so a
// condition that reads only the controller's own zones lowers; one that
// reads the source object (Self/Other, CheckSVar$ on the source, Solved)
// refuses, since the kernel's graveyard offer has no source slot.

// graveActCondOK reports whether c reads nothing of the source: the
// controller's graveyard (delirium, threshold), whose turn it is, or a
// count of cards/permanents matching a filter with no source-relative term.
func graveActCondOK(c lCond) bool {
	switch c.Kind {
	case cThreshold, cYourTurn, cOpponentsTurn:
		return true
	case cValue:
		if !c.B.isConst() {
			return false
		}
		return c.A.Kind == vGraveTypes && c.A.Mul == 0 && c.A.Div == 0 && c.A.Add == 0 && !c.A.Neg ||
			graveCountOK(c.A) || scryTallyOK(c.A) // value_scrytally.go
	case cPresent:
		return c.A.isConst() && graveFilterOK(c.F)
	}
	return false
}

// graveCountOK accepts the sums of vCount / vCountGraveyard / vCountExile
// leaves presentZoneCond builds, each over a source-independent filter.
func graveCountOK(v lValue) bool {
	if v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return false
	}
	switch v.Kind {
	case vCount, vCountGraveyard, vCountExile:
		return graveFilterOK(v.F)
	case vSum:
		for _, s := range v.Sub {
			if !graveCountOK(s) {
				return false
			}
		}
		return len(v.Sub) != 0
	}
	return false
}

// graveFilterOK: an allowlist of clause terms that read only the object
// itself and the controller (never the source, a target or a choice).
func graveFilterOK(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 {
		return false
	}
	const okFlags = fToken | fNonToken | fBasic | fLegendary
	for _, c := range f.Clauses {
		if c.Flags&^okFlags != 0 || c.CmpV != nil || c.CmpX != "" || c.Unblocked || c.SpellBase {
			return false
		}
		switch c.CmpStat {
		case cmpNone, cmpPower, cmpToughness, cmpCMC, cmpMultiColor, cmpMonoColor, cmpHistoric:
		default:
			return false
		}
	}
	return true
}
