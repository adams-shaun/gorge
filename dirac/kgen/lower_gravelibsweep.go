package kgen

// originGraveLibrary lowers a ChangeZoneAll over Origin$ Graveyard,Library
// into exile for the resolving player (Doomsday's "exile the rest": Defined$
// You, ChangeType$ Card.!IsRemembered). gorge effChangeZoneAll walks the
// origins zone-major inside one zone batch -- the player's graveyard, then
// their library top first -- and moves every match (zone_changeall.go's
// sweep loop); the kernel's OpChangeZoneAllEx MvGraveExile sweep with
// kop.CzGraveLibrary walks the same two lists in that order. Only that
// shape: another destination (a library put-back owes gorge's placement
// order and shuffle), another player or any rider refuses by name.
func (lc *lowerCtx) originGraveLibrary(ps *params) error {
	for _, k := range sortedParamKeys(ps.m) {
		switch k {
		case "Origin", "Destination", "ChangeType", "Defined", "SubAbility":
		default:
			if !ignoredParams[k] {
				return refuse("ChangeZoneAll Origin$ Graveyard,Library with %s$", k)
			}
		}
	}
	if ps.str("Destination") != "Exile" || ps.str("Defined") != "You" {
		return refuse("ChangeZoneAll Origin$ Graveyard,Library outside Defined$ You, Destination$ Exile")
	}
	f, err := lc.filter(ps.str("ChangeType"))
	if err != nil {
		return err
	}
	if len(f.Clauses) == 0 || f.Players != 0 {
		return refuse("ChangeZoneAll Origin$ Graveyard,Library filter shape")
	}
	lc.emit(lInstr{Op: "ChangeZoneAll", F: f, C: zExile, Obj: lRef{Kind: rYou}, CS: "from=Graveyard,Library",
		Note: "the player's graveyard, then their library top first, into exile"})
	return nil
}
