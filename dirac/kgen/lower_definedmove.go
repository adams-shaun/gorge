package kgen

import (
	"strings"
)

// definedMoveInert consumes the ChangeNum$ 1 / Mandatory$ True / DefinedPlayer$
// an object move carries (Lembas' "its owner shuffles it into their library";
// Virtue of Persistence's targeted return; Athreos, God of Passage's
// DefinedPlayer$ You beside Defined$ TriggeredNewCardLKICopy; Nashi, Moon's
// Legacy's beside ValidTgts$): gorge's object path moves the Defined$ or
// chosen targets and never reads them -- only the hidden-origin pickers and
// the library fetch dispatch do (zone_hand.go:21/150/171, zone_hidden.go:45/178,
// zone_change.go:694; moveDefinedLibraryObjects refuses the shape and
// searches). Any other count, a move without Defined$ or ValidTgts$,
// or a move from a hand, leaves them unread. Mandatory$ is inert only for a
// Defined$ move; the target path reads it itself. A library origin leaves
// DefinedPlayer$ unread: beside it gorge's library fetch dispatch searches
// the named player's library, a different move kgen refuses.
func definedMoveInert(ps *params, origin int) {
	if origin == zHand {
		return
	}
	_, defined := ps.get("Defined")
	// ValidTgts$ presence only: the target path reads the selector itself.
	_, targeted := ps.m["ValidTgts"]
	if !defined && !targeted {
		return
	}
	if cn, ok := ps.get("ChangeNum"); ok && strings.TrimSpace(cn) == "1" {
		ps.consume("ChangeNum")
	}
	if defined && ps.is("Mandatory", "True") {
		ps.consume("Mandatory")
	}
	if origin == zLibrary {
		return // gorge's library fetch dispatch beside it searches instead
	}
	ps.consume("DefinedPlayer")
}

// thisAndTarget lowers ThisDefinedAndTgts$ Self on a shuffled
// battlefield-to-library move of one target (Floodpits Drowner's "shuffle
// this and target creature ... into their owners' libraries"; gorge
// zone_change.go appends the Self extra after the targets and shuffles
// each owner once afterwards) to OpToLibraryPair. Any other value or move
// shape refuses.
func (lc *lowerCtx) thisAndTarget(ps *params, origin, dest int, flags []string, counterKind string, override *lOverride) (bool, error) {
	v, ok := ps.get("ThisDefinedAndTgts")
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(v) != "Self" {
		return false, nil // alsoSelf (lower_zone.go) refuses another value by name
	}
	if origin != zBattlefield || dest != zLibrary || !ps.is("Shuffle", "True") {
		return false, nil // alsoSelf (lower_zone.go) handles a targeted return
	}
	if len(flags) != 0 || counterKind != "" || override != nil || ps.is("Reveal", "True") {
		return true, refuse("ThisDefinedAndTgts$ %q shape", v)
	}
	if _, ok := ps.get("ValidTgts"); !ok {
		return true, refuse("ThisDefinedAndTgts$ without ValidTgts$")
	}
	if _, ok := ps.get("LibraryPosition"); ok {
		return true, refuse("ThisDefinedAndTgts$ with LibraryPosition$")
	}
	obj, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	if obj.Kind != rTarget {
		return true, refuse("ThisDefinedAndTgts$ beside %s", obj)
	}
	if t := lc.ab.Targets[obj.N]; !t.Max.isConst() || t.Max.N != 1 {
		return true, refuse("ThisDefinedAndTgts$ beside several targets")
	}
	ps.consume("Shuffle")
	lc.emit(lInstr{Op: "ToLibraryPair", Obj: obj, Note: "the target, then this, shuffled in"})
	return true, nil
}
