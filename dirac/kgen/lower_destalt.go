package kgen

// ChangeZone's unconditional DestinationAlternative$ (no DestAltSVar$, no
// AlternativeDecider$): Forge's ChangeZoneEffect.handleAltDest leaves
// altDestOpt true and asks the activator whether the card takes the primary
// destination. gorge models two shapes, and each lowers here:
//
//   - top or bottom of a library (DestinationAlternative$ Library with
//     LibraryPositionAlternative$ -1 over a top primary): the controller's
//     "primary"/"bottom" ask (engine/effects zone_positionchoice.go). On the
//     object path (Not Forgotten, Arashin Sovereign) the card must still be
//     in its Origin$ graveyard; on the hand put-back (Dream Cache) it is
//     asked once after the cards move (handLibraryTail).
//   - hand or graveyard after a library search (Destination$ Hand,
//     DestinationAlternative$ Graveyard: Dina's Guidance): the searcher's
//     changezone_dest_alt ask after the pick, only when a card was found
//     (zone_search.go askUnconditionalDestination).
//
// Every other unconditional alternative stays unread, so chain's unread-params
// check still refuses it.

// libChoiceCtrlGrave is lowered ToLibrary's C for the controller's top or
// bottom of a graveyard card (build_ab.go: OpToLibraryChoice with
// kop.LibChoiceCtrlGrave).
const libChoiceCtrlGrave = 5

// unconditionalAlt reports a DestinationAlternative$ with neither a
// DestAltSVar$ condition nor an AlternativeDecider$, without marking a key
// read.
func unconditionalAlt(ps *params) bool {
	_, svar := ps.m["DestAltSVar"]
	_, ad := ps.m["AlternativeDecider"]
	return ps.peek("DestinationAlternative") != "" && !svar && !ad
}

// libraryPositionAlt is the top-or-bottom alternative shape.
func libraryPositionAlt(ps *params, pos int) bool {
	return unconditionalAlt(ps) && pos == 0 && ps.peek("DestinationAlternative") == "Library" &&
		ps.peek("LibraryPositionAlternative") == "-1"
}

// ctrlPositionChoice reads the object path's top-or-bottom shape: a card
// moved from a graveyard (a target, or the trigger's card). ok false leaves
// the parameters unread.
func ctrlPositionChoice(ps *params, origin, pos int, obj lRef) (int, bool, error) {
	if !libraryPositionAlt(ps, pos) {
		return 0, false, nil
	}
	if origin != zGraveyard {
		return 0, false, refuse("top-or-bottom DestinationAlternative$ from %s", zoneLabel[origin])
	}
	switch obj.Kind {
	case rTarget, rTriggered, rTriggeredNew:
	default:
		return 0, false, refuse("top-or-bottom DestinationAlternative$ of %s", refNames[obj.Kind])
	}
	ps.consume("DestinationAlternative", "LibraryPositionAlternative")
	return libChoiceCtrlGrave, true, nil
}

// handPosAsk reads the hand put-back's top-or-bottom shape (Dream Cache).
func handPosAsk(ps *params, pos int) bool {
	if !libraryPositionAlt(ps, pos) {
		return false
	}
	ps.consume("DestinationAlternative", "LibraryPositionAlternative")
	return true
}

// searchDestAlt reads a library search's hand-or-graveyard alternative
// (DestAltGrave): the controller's own library, to a hand, with no other
// zone election.
func searchDestAlt(ps *params, dest int, obj lRef, flags []string) ([]string, error) {
	if !unconditionalAlt(ps) {
		return flags, nil
	}
	if ps.peek("DestinationAlternative") != "Graveyard" || dest != zHand {
		return flags, nil // unread: refused by name
	}
	if _, ok := ps.m["LibraryPositionAlternative"]; ok {
		return flags, refuse("search DestinationAlternative$ with LibraryPositionAlternative$")
	}
	if _, ok := ps.m["OriginAlternative"]; ok || obj.Kind != rYou {
		return flags, refuse("search DestinationAlternative$ with OriginAlternative$/DefinedPlayer$")
	}
	ps.consume("DestinationAlternative")
	return append(flags, "DestAltGrave"), nil
}
