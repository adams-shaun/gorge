package kgen

import (
	"fmt"
	"strings"
)

// lower_remembersearcher.go: ChangeZone's RememberSearcher$ on a library
// search (gorge effSearchLibrary remembers the searching player, found card or
// not, CR 701.23b). The kernel's remembered set holds objects, so the player
// joins the remembered count (kop.ExtRememberSearcher) and the shape is exact
// only where nothing reads the set but its size: one search by the controller
// of their own library, no confirmation or zone election that could skip it
// (gorge remembers after those), and no object read of Remembered anywhere on
// the card (refuseRememberSearcherObjects).

const noteRememberSearcher = "remember searcher"

// rememberSearcher lowers RememberSearcher$ True ahead of the Search emitted
// next; obj is the searched library's owner, flags the search's flags.
func (lc *lowerCtx) rememberSearcher(ps *params, obj lRef, flags []string) error {
	v, ok := ps.get("RememberSearcher")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberSearcher %q", v)
	}
	if obj.Kind != rYou {
		// In a remembered-set card loop the searched library is the bound
		// card's controller's (Wave of Vitriol's DefinedPlayer$
		// ImprintedController): RefEach, resolved per pass.
		if obj.Kind != rEach || !lc.repeatRem {
			return refuse("RememberSearcher$ on another player's library")
		}
	}
	for _, f := range flags {
		if strings.HasPrefix(f, "alt=") {
			return refuse("RememberSearcher$ beside %s", f)
		}
		if f == "Confirm" && obj.Kind != rEach {
			return refuse("RememberSearcher$ beside %s", f)
		}
	}
	for _, in := range lc.ab.Prog {
		if in.Note == noteRememberSearcher {
			return refuse("a second RememberSearcher$ in one ability")
		}
	}
	for _, k := range []string{"ChooseFromDefined", "DefinedPlayer", "Chooser", "ForgetOtherRemembered"} {
		if _, ok := ps.m[k]; ok {
			// the repeated card's controller is both the searched library's
			// owner and the chooser (Wave of Vitriol); the search lowering
			// read both.
			if obj.Kind == rEach && (k == "DefinedPlayer" || k == "Chooser") && strings.TrimSpace(ps.m[k]) == "ImprintedController" {
				continue
			}
			return refuse("RememberSearcher$ with %s$", k)
		}
	}
	ps.consume("RememberSearcher")
	lc.emit(lInstr{Op: "RememberSearcher", Obj: obj, Note: noteRememberSearcher})
	return nil
}

// hasRememberedSearcher reports whether the current ability already lowered a
// RememberSearcher$ whose searcher is the repeated card's controller (the
// remembered-set card loop). Only then does the seat mask exist for a following
// Shuffle Defined$ Player.IsRemembered (Wave of Vitriol).
func (lc *lowerCtx) hasRememberedSearcher() bool {
	for _, in := range lc.ab.Prog {
		if in.Op == "RememberSearcher" && in.Obj.Kind == rEach {
			return true
		}
	}
	return false
}

// refuseRememberSearcherObjects refuses a card with a RememberSearcher$ writer
// that reads the remembered set other than by its size: the set holds a player
// in gorge and nothing in the kernel.
func (lc *lowerCtx) refuseRememberSearcherObjects() {
	writes := false
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			writes = writes || in.Note == noteRememberSearcher
		}
	}
	if !writes {
		return
	}
	noCount := func(s string) string { return strings.ReplaceAll(s, "RememberedCount", "") }
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			if in.Note != noteRememberSearcher && kremReads(noCount(in.String())) {
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "RememberSearcher$ beside a remembered-object read (the kernel counts the searcher, it cannot hold the player)")
				return
			}
		}
	}
	for _, s := range lc.out.Statics {
		if kremReads(noCount(fmt.Sprint(s.F))) || kremReads(noCount(fmt.Sprint(s.Cond))) {
			lc.refuseNode("card", "RememberSearcher$ beside a static remembered read")
			return
		}
	}
}
