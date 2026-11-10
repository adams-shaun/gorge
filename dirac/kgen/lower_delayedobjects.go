package kgen

// delayedObjOnly reports whether a delayed registration's RememberObjects$
// set holds objects only, so the kernel's object-only RefDelayedRemembered is
// exact. RememberedLKI is the objects-only LKI read; TriggeredCard,
// TriggeredCardLKICopy, TriggeredNewCardLKICopy, Targeted and ParentTarget
// each name one object.
func delayedObjOnly(ro string) bool {
	switch ro {
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy", "Targeted", "ParentTarget", "RememberedLKI", "SacrificedCards":
		return true
	}
	return false
}

// delayedSelf reports whether a delayed trigger's RememberObjects$ form
// remembers the source's own card, so a Transformed$ return of it is the
// card turning over (CR 712.14a), not some other DFC being put onto the
// battlefield "transformed".
func delayedSelf(tr *lTrigger, ro string) bool {
	if tr == nil || tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) != 1 || tr.F.Clauses[0].Flags&fSelf == 0 {
		return false
	}
	switch ro {
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy":
	default:
		return false
	}
	switch tr.Event {
	case "Dies", "Sacrificed":
		return true
	case "ChangesZone":
		return tr.Origin == zBattlefield && (tr.Dest == zGraveyard || tr.Dest == zExile || tr.Dest == zAny)
	case "Leaves":
		return tr.From == zBattlefield
	}
	return false
}
