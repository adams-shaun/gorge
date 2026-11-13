package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 401.4 arrangement of cards one effect puts at a library position
// together (kop.ExtLibPlace). gorge asks the cards' owner to order them
// (effects.libraryPlacementOrder: a KArrange, Kind "library_placement") when a
// ChangeZoneAll sweep puts cards in a library, or a ChangeZone does with two or
// more of one owner's cards, unless the order is random (RandomOrder$) or the
// library is shuffled afterwards (Shuffle$ True without NoShuffle$); a Hidden$
// ChangeZone never asks. The lowering brackets the move with the arm and the
// ask. A shape the kernel cannot bracket is refused by name.
//
// It reads its params with peek: this is a guard, not a consumer, and must not
// alter what the census counts as read.

// libPlaceArranges reports whether gorge may pose the arrangement for a
// ChangeZone or ChangeZoneAll to a library: not Hidden$ (a ChangeZone), not
// RandomOrder$, and not shuffled afterwards.
func libPlaceArranges(ps *params, api string) bool {
	if ps.peek("Destination") != "Library" || ps.peek("RandomOrder") == "True" {
		return false
	}
	if ps.peek("Shuffle") == "True" && ps.peek("NoShuffle") != "True" {
		return false
	}
	return api == "ChangeZoneAll" || ps.peek("Hidden") != "True"
}

// libPlaceAll brackets a ChangeZoneAll sweep into a library: its cards are
// placed on top (the only position the sweep lowers), so the arm is the top
// one. A LibraryPosition$ other than the top is refused.
func libPlaceAll(ps *params) (bracket bool, err error) {
	if !libPlaceArranges(ps, "ChangeZoneAll") {
		return false, nil
	}
	if p := ps.peek("LibraryPosition"); p != "" && p != "0" {
		return false, refuse("ChangeZoneAll LibraryPosition$ %q: the CR 401.4 arrangement is lowered for the top only", p)
	}
	return true, nil
}

// libPlaceTargets brackets a ChangeZone to a library whose object is a
// multi-object target slot (TargetMax$ past 1 or a variable count): the cards
// of one owner may number two or more. pos is the LibraryPosition$ (0 top, -1
// bottom). A single-object move (a lone target, Defined$ Self) never asks.
func (lc *lowerCtx) libPlaceTargets(ps *params, obj lRef, in lInstr, pos int, optional bool) (arm int, bracket bool, err error) {
	if in.Op != "ToLibrary" || obj.Kind != rTarget || !libPlaceArranges(ps, "ChangeZone") {
		return 0, false, nil
	}
	if mx := lc.ab.Targets[obj.N].Max; mx.Kind == vNone || mx.isConst() && mx.N <= 1 {
		return 0, false, nil
	}
	switch {
	case in.C != 0 && in.C != 1, pos != 0 && pos != -1:
		return 0, false, refuse("ChangeZone of several targets to a library position other than the top or bottom (the CR 401.4 arrangement)")
	case optional:
		return 0, false, refuse("Optional$ ChangeZone of several targets to a library (the CR 401.4 arrangement)")
	}
	return in.C, true, nil
}

// libPlaceDefinedSet refuses a ChangeZone to a library of a Defined$ set the
// kernel may not move whole: the moves of a remembered or valid set can number
// two or more of one owner's cards, and the arrangement brackets none of them.
func libPlaceDefinedSet(ps *params) error {
	if !libPlaceArranges(ps, "ChangeZone") {
		return nil
	}
	d := ps.peek("Defined")
	if d == "" || d == "Self" || d == "TopOfLibrary" || strings.HasPrefix(d, "Targeted") {
		return nil
	}
	return refuse("ChangeZone Defined$ %q to a library: the CR 401.4 arrangement of several cards is not lowered", d)
}

// emitLibPlace emits in between the arm and the ask of kop.ExtLibPlace.
func (lc *lowerCtx) emitLibPlace(arm int, in lInstr) {
	lc.emit(lInstr{Op: "LibPlace", C: arm, Note: "count the cards this move puts into libraries"})
	lc.emit(in)
	lc.emit(lInstr{Op: "LibPlace", C: kop.PlaceAsk, Note: "each owner of two or more orders them (CR 401.4)"})
}
