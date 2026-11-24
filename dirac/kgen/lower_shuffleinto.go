package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// shuffleInto lowers a ChangeZone of one known object into its owner's
// library with Shuffle$ True (gorge effChangeZone's object path: the move,
// on top -- the absent LibraryPosition$ default --, then
// objectPathShuffleOwners shuffles the owner's library). Origin$ Battlefield
// or Graveyard; Defined$ Self / TriggeredNewCard[LKICopy] / Enchanted or one
// target.
// LibraryPosition$ other than 0 refuses. A multi-object graveyard slot (TargetMin$ 0 with
// TargetMax$ N, gorge's "up to N target cards from a graveyard") lowers to
// OpMayShuffleMove: the chosen cards move on top in pick order, then each
// distinct owner's library shuffles once in referent order (gorge
// objectPathShuffleOwners), or, with ShuffleNonMandatory$ True, the controller
// may decline (then nothing moves). The slot may name any graveyard card
// (ValidTgts$ Card: each chosen card goes to its own owner's library) or
// restrict the candidates to one owner with TargetsWithDefinedController$
// ParentTarget or a filter whose every clause is YouOwn/YouCtrl. The emitted
// instruction's W names that independently named library, so an empty set
// still shuffles it (CR 701.24d); the owner-defined Card filter has no W.
func (lc *lowerCtx) shuffleInto(ps *params, origin int) error {
	if origin == zStack {
		return lc.spellToLibrary(ps)
	}
	if origin != zBattlefield && origin != zGraveyard {
		return refuse("shuffled move from %s", zoneLabel[origin])
	}
	bottom := false
	if lp, ok := ps.get("LibraryPosition"); ok {
		// LibraryPosition$ 0 is the default top placement the shuffle then
		// randomises (gorge placeTargetedLibraryObjects, objectPathShuffleOwed);
		// -1 is the bottom, which the shuffle then randomises from there
		switch strings.TrimSpace(lp) {
		case "0":
		case "-1":
			if origin != zBattlefield {
				return refuse("shuffled move from %s with LibraryPosition$", zoneLabel[origin])
			}
			bottom = true
		default:
			return refuse("shuffled move with LibraryPosition$")
		}
		ps.consume("LibraryPosition")
	}
	obj, err := lc.objOrTargetParent(ps, "Self", origin)
	if err != nil {
		return err
	}
	self, err := lc.alsoSelf(ps, obj)
	if err != nil {
		return err
	}
	named := lRef{}
	if obj.Kind == rTarget {
		t := &lc.ab.Targets[obj.N]
		named = shuffleNamedLibrary(t)
		if !(t.Min.isConst() && t.Min.N == 1 && t.Max.isConst() && t.Max.N == 1) {
			if bottom {
				return refuse("shuffled move of several targets with LibraryPosition$")
			}
			if self {
				return refuse("ThisDefinedAndTgts$ Self with several targets")
			}
			return lc.shuffleIntoMulti(ps, origin, obj, t)
		}
	}
	switch obj.Kind {
	// rEquipped is Defined$ Enchanted / Equipped / AttachedTo: gorge's
	// definedSpecEquipped (engine/effects/context.go:1306-1328) names the
	// source's bearer (its AttachedTo, or LastBearer for a source off the
	// battlefield), and the shuffle tail's objectPathShuffleOwners
	// (engine/effects/zone_library.go:83-97) shuffles that object's owner's
	// library after the move -- or nothing when the source has no bearer.
	case rSelf, rTriggered, rTriggeredNew, rEquipped:
	case rTarget:
	default:
		return refuse("shuffled move of %s", obj)
	}
	if obj.Kind == rTriggered && origin != zBattlefield {
		obj.Kind = rTriggeredNew
	}
	cs := "TopShuffle"
	switch {
	case self && bottom:
		return refuse("shuffled move of the source and a target with LibraryPosition$")
	case self:
		cs = "TopShuffleSelf"
	case bottom:
		cs = "BottomShuffle"
	}
	lc.emit(lInstr{Op: "ToLibrary", Obj: obj, W: named, CS: cs + "|from=" + zoneLabel[origin]})
	return nil
}

// shuffleIntoMulti lowers the multi-object graveyard slot of shuffleInto
// (OpMayShuffleMove).
func (lc *lowerCtx) shuffleIntoMulti(ps *params, origin int, obj lRef, t *lTarget) error {
	if origin != zGraveyard {
		return refuse("shuffled move of several targets from %s", zoneLabel[origin])
	}
	if !t.Min.isConst() || t.Min.N != 0 || !t.Max.isConst() || t.Max.N < 1 || t.Max.N > 8 {
		return refuse("shuffled move of several targets with TargetMin$ %s TargetMax$ %s", t.Min, t.Max)
	}
	if t.AtRes || t.Unique || t.SameCtrl || t.DiffCtrl || t.SharedCrType || t.DistinctName || t.MaxMV != 0 || t.Divided != 0 || t.Spell {
		return refuse("shuffled move of several targets with target modifiers")
	}
	if t.ParentPlus1 == 0 && (t.F == nil || len(t.F.Clauses) == 0) {
		return refuse("shuffled move of several owners")
	}
	may := 0
	if ps.is("ShuffleNonMandatory", "True") {
		may = 1
	}
	// The may election is the effect controller's (gorge asks c.Controller),
	// not the parent player target's; Obj carries that asker. A mandatory
	// shuffle asks nobody, so it carries no player operand.
	// W is the mandatory form's named library (CR 701.24d); the may election
	// does nothing for an empty set, so it carries none and its LIR is unchanged.
	var asker, named lRef
	if may != 0 {
		asker = lRef{Kind: rYou}
	} else {
		named = shuffleNamedLibrary(t)
	}
	lc.emit(lInstr{Op: "MayShuffleMove", Obj: asker, W: named, C: obj.N, A: cst(may)})
	return nil
}

// shuffleNamedLibrary names the library a shuffled ChangeZone names
// independently of the chosen cards (CR 701.24d: it is shuffled even when
// the set is empty). TargetsWithDefinedController$ ParentTarget names the
// parent player's library; a filter whose every alternative is YouCtrl or
// YouOwn names the controller's. An owner-defined filter (Turn the Earth's
// Card) names no library when it chose no cards, so its Ref is none.
func shuffleNamedLibrary(t *lTarget) lRef {
	switch {
	case t == nil:
		return lRef{}
	case t.ParentPlus1 == int(kop.ParentTrigPl):
		return lRef{Kind: rTriggeredPlayer}
	case t.ParentPlus1 != 0:
		return lRef{Kind: rTarget, N: t.ParentPlus1 - 1}
	case namesYourLibrary(t.F):
		return lRef{Kind: rYou}
	}
	return lRef{}
}

// namesYourLibrary reports whether every alternative of f scopes its objects
// to the resolving controller through a YouCtrl or YouOwn qualifier. Any
// unscoped alternative answers false, so the owner-defined "those cards"
// filters never gain a controller library.
func namesYourLibrary(f *lFilter) bool {
	if f == nil || len(f.Clauses) == 0 || f.Players != 0 {
		return false
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoYou && c.Owner != whoYou {
			return false
		}
	}
	return true
}

// spellToLibrary lowers the resolving spell's own shuffle into its owner's
// library (ChangeZone Origin$ Stack Destination$ Library Shuffle$ True
// Defined$ Parent, the sub-ability of Blue Sun's Zenith): OpExt ExtSpellToLib.
// Gorge's Defined$ Parent names the ability's source, here the spell. Only a
// spell's own chain lowers; a library position or any other selector refuses.
func (lc *lowerCtx) spellToLibrary(ps *params) error {
	if lc.ab.Kind != akSpell {
		return refuse("shuffled move from Stack outside a spell")
	}
	if d := ps.str("Defined"); d != "Parent" && d != "Self" {
		return refuse("shuffled move from Stack of %q", d)
	}
	if ps.str("Destination") != "Library" {
		return refuse("shuffled move from Stack to %s", ps.str("Destination"))
	}
	if lp, ok := ps.get("LibraryPosition"); ok && strings.TrimSpace(lp) != "0" {
		return refuse("shuffled move from Stack with LibraryPosition$")
	}
	ps.consume("Defined", "Origin", "Destination", "Shuffle")
	lc.emit(lInstr{Op: "SpellToLib", Note: "the resolving spell shuffles itself into its owner's library"})
	return nil
}
