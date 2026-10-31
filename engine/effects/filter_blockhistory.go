package effects

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// The block-history filter words: the referent-based readers of the this-turn
// block record (state.Object.BlockedByThisTurn / BlockedThisTurn, CR 509.1)
// plus the two live source-relative words that share their dispatch.
//
//	blockedByValidThisTurn <arg>  the object was blocked by a creature <arg> names
//	blockedValidThisTurn <arg>    the object blocked a creature <arg> names
//	blockedByRemembered           ... by a remembered creature
//	blockedRemembered             ... a remembered creature
//	blockedBySourceLKI            the object (still attacking) was blocked by
//	                              the source, even though the source has left
//
// <arg> is a referent (Targeted / ParentTarget: ANY-OF the bound object
// targets) or a valid spec resolved against the history ids' objects. History
// ids survive a zone move (the engine keeps ObjID), so a blocker destroyed
// before the read still names itself and a dead attacker still matches a type
// spec. blockedBySourceLKI reads the history rather than the live BlockedBy
// because a departed blocker is tombstoned out of BlockedBy (CR 509.1h: the
// attacker stays blocked). IsAttacking limits it to the current combat; on a
// turn with two combat phases it still matches a creature the source blocked
// in the first one (no per-combat history is kept).

type blockHistoryWordCode uint16

const (
	blockHistoryByValid blockHistoryWordCode = iota + 1
	blockHistoryValid
	blockHistoryByRemembered
	blockHistoryRemembered
	blockHistoryBySourceLKI
)

var blockHistoryWordCodes = state.NewStrCodes(
	state.StrEntry[blockHistoryWordCode]{Key: "blockedByValidThisTurn", Val: blockHistoryByValid},
	state.StrEntry[blockHistoryWordCode]{Key: "blockedValidThisTurn", Val: blockHistoryValid},
	state.StrEntry[blockHistoryWordCode]{Key: "blockedByRemembered", Val: blockHistoryByRemembered},
	state.StrEntry[blockHistoryWordCode]{Key: "blockedRemembered", Val: blockHistoryRemembered},
	state.StrEntry[blockHistoryWordCode]{Key: "blockedBySourceLKI", Val: blockHistoryBySourceLKI},
)

type blockHistoryArgCode uint16

const (
	blockHistoryArgTargeted blockHistoryArgCode = iota + 1
	blockHistoryArgSelf
)

var blockHistoryArgCodes = state.NewStrCodes(
	state.StrEntry[blockHistoryArgCode]{Key: "Targeted", Val: blockHistoryArgTargeted},
	state.StrEntry[blockHistoryArgCode]{Key: "ParentTarget", Val: blockHistoryArgTargeted},
	state.StrEntry[blockHistoryArgCode]{Key: "Card.StrictlySelf", Val: blockHistoryArgSelf},
)

// blockHistoryWord classifies the block-history words. The two valid-spec
// words arrive as ONE space-joined token ("blockedByValidThisTurn Zombie");
// only the argument shapes the corpus uses are recognised -- a referent,
// Card.StrictlySelf, or a base type word with a fully known qualifier chain.
// Anything else is not ok and stays wordUnknown (fails closed).
func blockHistoryWord(p string) (wordKind, string, bool) {
	name, arg, spaced := strings.Cut(p, " ")
	var code blockHistoryWordCode
	if spaced {
		code = blockHistoryWordCodes.Code(name)
		if code != blockHistoryByValid && code != blockHistoryValid {
			return wordUnknown, "", false
		}
		arg = strings.TrimSpace(arg)
		if !blockHistoryArgKnown(arg) {
			return wordUnknown, "", false
		}
	} else {
		code = blockHistoryWordCodes.Code(p)
	}
	switch code {
	case blockHistoryByValid:
		return wordBlockedByValid, arg, spaced
	case blockHistoryValid:
		return wordBlockedValid, arg, spaced
	case blockHistoryByRemembered:
		return wordBlockedByRemembered, "", !spaced
	case blockHistoryRemembered:
		return wordBlockedRemembered, "", !spaced
	case blockHistoryBySourceLKI:
		return wordBlockedBySourceLKI, "", !spaced
	}
	return wordUnknown, "", false
}

// blockHistoryArgKnown reports whether arg is a recognised referent argument.
func blockHistoryArgKnown(arg string) bool {
	if arg == "" || strings.ContainsAny(arg, "+,! ") {
		return false
	}
	if blockHistoryArgCodes.Code(arg) != 0 {
		return true
	}
	base, _, _ := strings.Cut(arg, ".")
	if !predicateTypeWords[base] {
		return false
	}
	return len(UnknownPredicates(arg)) == 0
}

// blockHistoryReferentIsTarget reports whether the valid-word argument names
// the bound targets rather than a spec.
func blockHistoryReferentIsTarget(arg string) bool {
	return blockHistoryArgCodes.Code(arg) == blockHistoryArgTargeted
}

// blockHistoryNames reports whether any id in history is named by the
// referent: an object target (ANY-OF, so Sorrow's Path's two targets are a
// union), or a spec match with sc kept so Card.StrictlySelf resolves against
// the source.
func blockHistoryNames(g *state.Game, history []state.ObjID, arg string, sc *SpecContext) bool {
	if blockHistoryReferentIsTarget(arg) {
		ts, _ := sc.TargetBinding()
		for _, t := range ts {
			if !t.IsPlayer && slices.Contains(history, t.Obj) {
				return true
			}
		}
		return false
	}
	for _, id := range history {
		if other := g.Obj(id); other != nil && matchesObjectPtr(g, arg, other, sc) {
			return true
		}
	}
	return false
}

// blockHistoryRememberedNames reports whether any id in history is in the
// Remembered union.
func blockHistoryRememberedNames(g *state.Game, history []state.ObjID, sc *SpecContext) bool {
	return anyRemembered(g, sc, func(id state.ObjID) bool { return slices.Contains(history, id) })
}

// anyRemembered walks the Remembered referent -- the resolution's Remembered
// set UNIONED with the source object's event-backed Remembered, objects only
// -- and reports whether fn accepts any id. IsRemembered and the
// *Remembered block-history words share it, so the two cannot disagree on
// what "remembered" is. A trigger's fire-time capture in the resolution set
// is filtered through selectRemembered, the Defined$ Remembered rule; the
// persistent half is effectiveSourceRemembered, the same pending-referent
// rule the Defined$ read resolves under (a source that re-entered the
// battlefield while its trigger waited answers the old object's snapshot).
func anyRemembered(g *state.Game, sc *SpecContext, fn func(state.ObjID) bool) bool {
	src := g.Obj(sc.Source)
	persistent := effectiveSourceRemembered(g, sc.TriggerContext, src)
	resolution := sc.Remembered
	if len(sc.Captured) > 0 {
		resolution = selectRemembered(sc.Remembered, sc.Captured, sc.DelayedRemembered, persistent, sc.RememberWriterAbove)
	}
	for _, t := range resolution {
		if !t.IsPlayer && fn(t.Obj) {
			return true
		}
	}
	for _, t := range persistent {
		if !t.IsPlayer && fn(t.Obj) {
			return true
		}
	}
	return false
}

// blockHistoryMatches evaluates the block-relation words for candidate o.
func blockHistoryMatches(kind wordKind, key string, g *state.Game, o *state.Object, sc *SpecContext) bool {
	switch kind {
	case wordBlockingSource:
		// Forge's blockingSource: the object is a creature blocking the
		// source. BlockedBy is recorded on the attacked object, so the
		// source's BlockedBy names its blockers; this object is one of them.
		s := g.Obj(sc.Source)
		return s != nil && containsID(s.BlockedBy, o.ID)
	case wordBlockedBySource:
		// Forge's blockedBySource: the object is being blocked by the source
		// -- the source is one of THIS object's blockers.
		return containsID(o.BlockedBy, sc.Source)
	case wordBlockedByValid:
		return blockHistoryNames(g, o.BlockedByThisTurn, key, sc)
	case wordBlockedValid:
		return blockHistoryNames(g, o.BlockedThisTurn, key, sc)
	case wordBlockedByRemembered:
		return blockHistoryRememberedNames(g, o.BlockedByThisTurn, sc)
	case wordBlockedRemembered:
		return blockHistoryRememberedNames(g, o.BlockedThisTurn, sc)
	case wordBlockedBySourceLKI:
		return sc.Source != 0 && o.IsAttacking && slices.Contains(o.BlockedByThisTurn, sc.Source)
	}
	return false
}

// blockHistoryBound reports whether the referent a block-history word reads
// is bound; an unbound one must fail closed beneath '!' too. The live
// source-relative words keep their historical unconditional behaviour.
func blockHistoryBound(g *state.Game, kind wordKind, key string, sc *SpecContext) bool {
	switch kind {
	case wordBlockedByValid, wordBlockedValid:
		if blockHistoryReferentIsTarget(key) {
			_, ok := sc.TargetBinding()
			return ok
		}
		return true
	case wordBlockedByRemembered, wordBlockedRemembered:
		return anyRemembered(g, sc, func(state.ObjID) bool { return true })
	case wordBlockedBySourceLKI:
		return sc.Source != 0
	}
	return true
}
