package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ChangeZone Defined$ TopOfLibrary Origin$ Library
// Optional$ True or absent, the else branch after an optional PeekAndReveal
// (Gathering Stone, Traveling Botanist: to the graveyard; Bucolic Ranch: to
// the bottom) and Vivien's Grizzly's conditional moves (to the hand, to the
// bottom). Gorge dispatches Origin$ Library plus a Defined$ selector to
// moveDefinedLibraryObjects (zone_search.go), not the ordinary object path and
// not the library search: an empty library is consumed with no ask and no move
// (kop.MayGuardLibraryTop; the absent-Optional shape skips it through the
// library-size condition, no ask), otherwise one yes / no ask, and a yes moves
// the top card with no shuffle. Defined$ TopOfLibrary is read here and nowhere
// else (defined() does not know it: every other API's TopOfLibrary has no
// kernel arm).

// topLibraryUnread are the keys that change what gorge's move does; none may
// be present.
var topLibraryUnread = [...]string{
	"AlternativeDecider", "Attacking", "AtRandom", "ChangeNum", "ChangeType", "Chooser",
	"DefinedPlayer", "DifferentNames", "Duration", "ExileFaceDown", "ForgetOtherRemembered",
	"GainControl", "Hidden", "Mandatory", "RememberChanged", "RememberLKI", "RememberTargets",
	"Reveal", "Shuffle", "StaticEffect", "Tapped", "ThisDefinedAndTgts", "Transformed",
	"Unearth", "ValidTgts", "WithCountersType",
}

// topLibraryMove lowers the shape above. The caller has read Origin$,
// Destination$ and LibraryPosition$ (origin, dest, pos) and the flags /
// counters / override from the other keys; every check runs before the first
// emit so a refusal leaves no partial program.
func (lc *lowerCtx) topLibraryMove(ps *params, origin, dest, pos int, flags []string, counterKind string, override *lOverride) error {
	where := func(why string) error {
		return refuse("ChangeZone Defined$ TopOfLibrary %s->%s with %s", zoneLabel[origin], zoneLabel[dest], why)
	}
	if origin != zLibrary {
		return where("Origin$ other than Library")
	}
	// Optional$ True asks the controller yes / no over the whole move; its
	// absence moves unconditionally (gorge cz.OptionalTrue false), so the
	// empty-library no-op gates the move with the library size instead of an
	// ask (gorge: the empty fetch list is still a direct fetch, and it moves
	// nothing).
	if v, ok := ps.get("Optional"); ok && strings.TrimSpace(v) != "True" {
		return refuse("ChangeZone Defined$ TopOfLibrary Optional$ %q", v)
	}
	if ps.is("ExileFaceDown", "True") {
		return lc.topLibraryExileFaceDown(ps, origin, dest, pos, flags, counterKind, override, where)
	}
	for _, k := range topLibraryUnread {
		if _, ok := ps.m[k]; ok {
			return where(k + "$")
		}
	}
	if len(flags) != 0 || counterKind != "" || override != nil {
		return where("flags, counters or an override")
	}
	in := lInstr{Obj: lRef{Kind: rLibTop}}
	_, hasPos := ps.m["LibraryPosition"]
	switch {
	case dest == zGraveyard && !hasPos:
		in.Op = "ToGraveyard"
		in.Note = "the top card of the library to the graveyard"
	case dest == zLibrary && hasPos && pos == -1:
		in.Op, in.C = "ToLibrary", 1
		in.Note = "the top card of the library to the bottom"
	case dest == zHand && !hasPos:
		// a plain move, not a return: the card had no origin the move owes
		// (gorge settleChangeZoneMove: one MoveZone library->hand, no
		// shuffle; CR 400.7 is a new object in either engine)
		in.Op, in.C = "Return", zHand
		in.Note = "the top card of the library to the hand"
	default:
		return where("Destination$ other than the Hand, Graveyard or the bottom of the library")
	}
	optional := ps.m["Optional"] != ""
	if !optional {
		// gorge: an empty library's TopOfLibrary fetch moves nothing, with
		// no ask; the gate is the controller's library size, read at the
		// move (the same moment gorge resolves the selector). The SkipIf
		// runs what follows while its condition holds, so the proceed
		// condition is the non-empty library.
		lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue,
			A:  lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}},
			Op: opGT, B: cst(0)}, Skip: 1, Note: "the library is empty: nothing moves"})
	}
	if err := lc.mayGate(ps, kop.MayGuardLibraryTop, lRef{Kind: rYou}); err != nil {
		return err
	}
	lc.emit(in)
	return nil
}

// topLibraryExileFaceDown lowers the hidden-library face-down exile shape
// (Necropotence): "exile the top card of your library face down" then a
// later effect reads it. gorge's moveDefinedLibraryObjects asks the same
// empty-library question, moves the card and applyMoveFaceDown stamps the
// blind exiled_with_face_down marker with RememberChanged$ keeping it for
// the delayed return; the kernel marks the remembered cards after the move
// with ExtExileFaceDown (dirac/zone_exilefacedownmove.go).
func (lc *lowerCtx) topLibraryExileFaceDown(ps *params, origin, dest, pos int, flags []string, counterKind string, override *lOverride, where func(string) error) error {
	if dest != zExile {
		return where("ExileFaceDown$ to " + zoneLabel[dest])
	}
	if _, ok := ps.m["LibraryPosition"]; ok || pos != 0 {
		return where("LibraryPosition$")
	}
	if _, ok := ps.get("Optional"); ok {
		return where("Optional$")
	}
	if _, ok := ps.m["WithMayLook"]; ok {
		return where("WithMayLook$")
	}
	if !ps.is("RememberChanged", "True") {
		return where("ExileFaceDown$ without RememberChanged$ True")
	}
	for _, k := range topLibraryUnread {
		if k == "ExileFaceDown" || k == "RememberChanged" {
			continue
		}
		if _, ok := ps.m[k]; ok {
			return where(k + "$")
		}
	}
	if len(flags) != 1 || flags[0] != "Remember" || counterKind != "" || override != nil {
		return where("flags, counters or an override")
	}
	// no Optional$: an empty library moves nothing (the same library-size
	// gate the plain top-library fetch emits).
	lc.emit(lInstr{Op: "SkipIf", Cond: lCond{Kind: cValue,
		A:  lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}},
		Op: opGT, B: cst(0)}, Skip: 1, Note: "the library is empty: nothing moves"})
	lc.emit(lInstr{Op: "Exile", Obj: lRef{Kind: rLibTop}, CS: "Remember"})
	lc.emit(lInstr{Op: "ExileFaceDown", Note: "the exiled top card is face down"})
	return nil
}
