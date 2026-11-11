package kgen

// thisTurnActivatedValue lowers Count$ThisTurnActivated_<spec> (the /Op
// suffix is already peeled). Only gorge's AbilitiesActivatedThisTurnMatching
// reading of Activated.YouCtrl+inZoneGraveyard is lowered
// (kop.ExThisTurnActGrave): the controller's activations
// whose source sits in a graveyard now (Laboratory Drudge). The ledger
// records every non-mana activation, so any property the kernel does not
// read per source at read time (Equip, Cycling, Loyalty, a targeting
// clause) refuses with a named reason.
func (lc *lowerCtx) thisTurnActivatedValue(spec string) (lValue, error) {
	if spec == "Activated.YouCtrl+inZoneGraveyard" {
		return lValue{Kind: vThisTurnActGrave}, nil
	}
	if spec == "Activated.Exhaust+YouCtrl" {
		// Elvish Refueler's this turn's Exhaust activations, read by its
		// Activations static's LT1 gate (dirac/turn_facts.go).
		return lValue{Kind: vThisTurnActExhaust}, nil
	}
	return lValue{}, refuse("other:count ThisTurnActivated %q: only Activated.YouCtrl+inZoneGraveyard and Activated.Exhaust+YouCtrl are lowered", spec)
}
