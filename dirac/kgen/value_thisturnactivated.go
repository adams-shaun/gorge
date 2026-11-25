package kgen

// thisTurnActivatedValue lowers Count$ThisTurnActivated_<spec> (the /Op
// suffix is already peeled). Only gorge's AbilitiesActivatedThisTurnMatching
// reading of Activated.YouCtrl+inZoneGraveyard is lowered
// (kop.ExThisTurnActGrave): the controller's activations
// whose source sits in a graveyard now (Laboratory Drudge); Activated.Exhaust+
// YouCtrl and Activated.Equip+YouCtrl read the per-turn presence bits
// (kop.ExThisTurnActExhaust N 0 and N 1). The ledger
// records every non-mana activation, so any property the kernel does not
// read per source at read time (Equip, Cycling, Loyalty, a targeting
// clause) refuses with a named reason.
func (lc *lowerCtx) thisTurnActivatedValue(spec string) (lValue, error) {
	if spec == "Activated.YouCtrl+inZoneGraveyard" {
		return lValue{Kind: vThisTurnActGrave}, nil
	}
	if spec == "Activated.Exhaust+YouCtrl" {
		// Elvish Refueler's this turn's Exhaust activations, read by its
		// Activations static's LT1 gate (dirac/static_turnbits.go).
		return lValue{Kind: vThisTurnActExhaust}, nil
	}
	if spec == "Activated.Equip+YouCtrl" {
		// Kili the Resourceful's "first equip ability you activate each turn"
		// gate: the same turn fact, its Equip arm (kop.ExThisTurnActExhaust N 1,
		// dirac/static_turnbits.go thisTurnActEquip).
		return lValue{Kind: vThisTurnActExhaust, N: 1}, nil
	}
	return lValue{}, refuse("other:count ThisTurnActivated %q: only Activated.YouCtrl+inZoneGraveyard, Activated.Exhaust+YouCtrl and Activated.Equip+YouCtrl are lowered", spec)
}
