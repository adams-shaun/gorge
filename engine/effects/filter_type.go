package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// canReceiveCounter answers Forge's Card.canReceiveCounters: any permanent can
// have a counter of any kind put on it, a noncreature's +1/+1 counter included
// (CR 122.1a only limits what the counter does). A static that stops a counter
// is applied by the put itself.
func canReceiveCounter(o *state.Object) bool {
	return o != nil && o.Zone == state.ZBattlefield && o.Face() != nil
}

// hasType reads a printed type plus Changeling's type-defining ability. The
// rules package supplies SpecContext.Types when a layer-derived type list is
// available; this fallback remains deliberately useful to effects, which sits
// below rules and cannot import the layer engine.
func hasType(o *state.Object, t string) bool { return hasTypeID(o, t, 0) }

// hasTypeID is hasType with t's precompiled cards.InternTypeWord ordinal (0 =
// none): the printed type-line test is one bit test when the face is bound.
func hasTypeID(o *state.Object, t string, id cards.TypeWordID) bool {
	d, _ := hasTypePrinted(o, t, id)
	if d != typeUndecided {
		return d == typeYes
	}
	// Intrinsic type-defining abilities, answered in every zone (CR 613.4a):
	// Changeling's keyword and the characteristic-defining
	// AddAllCreatureTypes$ True static (Mistform Ultimus). Both go through
	// the positive subtype vocabulary, so a non-creature word (Arcane,
	// Alara, Ajani) can never leak, and neither materialises subtypes into
	// the derived type list.
	return IntrinsicAllCreatureTypes(o) && changelingType(t)
}

// hasTypeSub is hasType with changelingType(t) supplied precomputed as sub
// (the compiled filter form classifies its type words once). Every function
// involved is pure, so reading sub first only skips the keyword probe for a
// word no Changeling can grant.
func hasTypeSub(o *state.Object, t string, id cards.TypeWordID, sub bool) bool {
	d, _ := hasTypePrinted(o, t, id)
	if d != typeUndecided {
		return d == typeYes
	}
	return sub && IntrinsicAllCreatureTypes(o)
}

type typeDecision uint8

const (
	typeUndecided typeDecision = iota
	typeYes
	typeNo
)

// hasTypePrinted is hasType up to (not including) its intrinsic-CDA tail:
// the bestow/reconfigure switches and the printed type line. Undecided means
// the answer is the CDA tail's, read off the returned face.
func hasTypePrinted(o *state.Object, t string, id cards.TypeWordID) (typeDecision, *cards.Face) {
	f := o.Face()
	if f == nil {
		return typeNo, nil
	}
	// CR 708.8/708.2: while a battlefield permanent is face down its printed
	// type line does not exist -- the object's type words are exactly its
	// folded face-down set (state.Object.FaceDownTypeWords: {Creature}, or
	// the FaceDownSetType$ replacement, Yedora's Forest). This is the one
	// gate every printed-face type fallback shares: without it, a trigger or
	// offer filter over a disguised/manifested permanent read the HIDDEN
	// face and matched the card's printed subtypes (Case of the Pilfered
	// Proof's "Whenever a Detective you control enters" counted a face-down
	// Basilica Stalker entering). The bestow/reconfigure switches below read
	// the printed face and cannot fire for a word that no longer exists, so
	// the gate runs before them; the Changeling tail in hasTypeID is skipped
	// by the same token (a decided No) -- a face-down permanent has no
	// abilities (CR 708.2) and its printed Changeling cannot grant one.
	if o.FaceDown && o.Zone == state.ZBattlefield {
		for _, w := range o.FaceDownTypeWords() {
			if strings.EqualFold(w, t) {
				return typeYes, f
			}
		}
		return typeNo, f
	}
	// CR 702.114e: a bestowed card attached to a creature is an Aura, not a
	// creature, in every filter read (Count$Valid, target offer, cost
	// candidates, statics' Affected$). Derived live state
	// (state.Object.BestowedAttached); the layer walk sees the same switch
	// through engine/rules/chars/types.go's bestowedTypeSwitch, and hasTypeCtx inherits
	// this gate through the hasType call below.
	//
	// CR 702.114c: a card cast with its bestow ability is an Aura SPELL, not a
	// creature spell -- the same switch one zone earlier, derived from the
	// stack zone and the pay-time FlagBestowed provenance
	// (state.Object.BestowedAuraSpell). This is what keeps a bestowed cast
	// from firing "whenever you cast a creature spell" triggers.
	if o.BestowedAttached() || o.BestowedAuraSpell() {
		if strings.EqualFold(t, "Aura") {
			return typeYes, f
		}
		if strings.EqualFold(t, "Creature") {
			return typeNo, f
		}
	}
	// CR 702.150c: a Reconfigure card attached to a creature is not a
	// creature, in the same every-filter-read sense (the target ask's
	// ValidTgts$ Creature, a Count$Valid Creature census, the combat
	// eligibility scans). Equipment and Artifact stay true -- they are the
	// printed face's own types and the attached form keeps them.
	if o.ReconfiguredAttached() && !(o.FaceDown && o.Zone == state.ZBattlefield) {
		if strings.EqualFold(t, "Creature") {
			return typeNo, f
		}
	}
	// CR 702.176a: a permanent cast for its impending cost is not a creature
	// while it has a time counter, so Creature and every creature subtype
	// answer no (the layer walk's chars/types.go impendingTypeSwitch strips
	// the same words). The enchantment type and its subtypes stay true, and
	// the decided No also skips the Changeling tail, which the layer path
	// likewise withholds from a dormant permanent.
	if o.ImpendingDormant() && (strings.EqualFold(t, "Creature") || changelingType(t)) {
		return typeNo, f
	}
	if f.TypeLineHas(t, id) {
		return typeYes, f
	}
	// CR 709.4c: a split card whose halves are combined has each card type
	// on either half (Commit // Memory is an instant and a sorcery card).
	if halves, ok := o.SplitHalves(); ok {
		for _, h := range halves {
			if h != f && h.TypeLineHas(t, id) {
				return typeYes, f
			}
		}
	}
	return typeUndecided, f
}

// typePredicate handles the legacy predicate-map entries whose meaning is a
// type test. Keeping them in one context-aware path ensures layer-4 derived
// types and Changeling apply consistently to both positive and negated forms.
func typePredicate(p string, g *state.Game, o *state.Object, sc SpecContext) (bool, bool) {
	switch typePredicateCodes.Code(string(p)) {
	case typePredicateSupertype:
		return hasTypeCtx(o, p, sc), true
	case typePredicateNonLand:
		return !hasTypeCtx(o, "Land", sc), true
	case typePredicateNonCreature:
		return !hasTypeCtx(o, "Creature", sc), true
	case typePredicateNonBasic:
		return !hasTypeCtx(o, "Basic", sc), true
	case typePredicateChosenType:
		t, _ := sourceChosen(g, sc)
		return t != "" && hasTypeCtx(o, t, sc), true
	case typePredicateIsNotChosenType:
		t, _ := sourceChosen(g, sc)
		return t != "" && !hasTypeCtx(o, t, sc), true
	case typePredicateChosenCtrl:
		return chosenCtrlMatches(g, o, sc.Source), true
	case typePredicateExiledWithSource:
		return exiledBySourceAt(g, o, sc.Source, sc.TriggerContext.SourceIncarnation), true
	}
	return false, false
}

// chosenCtrlMatches is the ONE implementation of the ChosenCtrl predicate
// (controlled by the source's secretly chosen player) the context-aware
// typePredicate path and the census-only predicates-map entry share, so the
// matcher and UnknownPredicates cannot drift. A source with no chosen player
// fails closed.
func chosenCtrlMatches(g *state.Game, o *state.Object, src state.ObjID) bool {
	s := g.Obj(src)
	if s == nil {
		return false
	}
	for _, t := range s.Chosen {
		if t.IsPlayer && o.Controller == t.Player {
			return true
		}
	}
	return false
}

// hasTypePredicateCtx matches a type word, or a space-separated multi-word
// subtype (Time Lord) as every one of its words. strings.Cut rather than
// strings.Fields: this is the filter hot path and must not allocate
// (TestSimpleFilterMatchingDoesNotAllocate).
func hasTypePredicateCtx(o *state.Object, t string, sc SpecContext) bool {
	return hasTypePredicateCtxPtr(o, t, &sc)
}

// hasTypePredicateCtxPtr is hasTypePredicateCtx through a pointer (see
// hasTypeCtxPtr).
func hasTypePredicateCtxPtr(o *state.Object, t string, sc *SpecContext) bool {
	if t == "" {
		return false
	}
	for {
		word, rest, more := strings.Cut(t, " ")
		if word != "" && !hasTypeCtxPtr(o, word, sc) {
			return false
		}
		if !more {
			return true
		}
		t = rest
	}
}

func hasTypeCtx(o *state.Object, t string, sc SpecContext) bool {
	return hasTypeCtxPtr(o, t, &sc)
}

// hasTypeCtxPtr is hasTypeCtx reading the context through a pointer, so the
// compiled predicate paths that already hold a *SpecContext do not copy the
// whole context per type test (see hasEffectiveNamePtr).
func hasTypeCtxPtr(o *state.Object, t string, sc *SpecContext) bool {
	return hasTypeCtxPtrID(o, t, 0, sc)
}

// hasTypeCtxPtrID is hasTypeCtxPtr with t's precompiled type-word ordinal
// (0 = none), which only the printed-face fallback reads.
func hasTypeCtxPtrID(o *state.Object, t string, id cards.TypeWordID, sc *SpecContext) bool {
	// ExtraTypes is the layer walk's accumulating type list for the ONE
	// object being matched: a plain value slice, deliberately not a callable
	// resolver. Any call made through a SpecContext field makes escape
	// analysis leak the whole context to the heap on every hot-path
	// construction (the statics/action hotspot pins measure exactly that),
	// while a slice field is read-only and allocation-free. It is checked
	// first because it is the layer walk's OWN types-so-far list, which can
	// differ from the published table mid-walk.
	if sc.ExtraTypes != nil && o.ID == sc.ExtraTypesOwner {
		for _, x := range sc.ExtraTypes {
			if strings.EqualFold(x, t) {
				return true
			}
		}
		// When the layer walk bound a types-so-far list, it is authoritative.
		// The published table is not consulted in that case: it may carry a type a
		// LATER effect grants, which would break the walk's ordering (engine/rules/layers.go
		// clears sc.DerivedTypes for the same reason, but this guard keeps the
		// contract even for a caller that sets ExtraTypes without clearing it).
		return false
	}
	// Outside the walk a published layer-4 entry makes the object's DERIVED
	// type list authoritative for type words: it already carries the printed
	// types the effect kept (rules' typeCharacteristics folds them in), so a
	// RemoveCardTypes$ cannot be resurrected by a fallback to the printed face,
	// while a granted word (a static's AddTypes$, AddAllCreatureTypes$) is
	// found exactly as the layer walk finds it. The semantic all-types marker
	// is the layer-4 result too, never a fresh read of the printed CDA.
	for _, d := range sc.Layers.DerivedTypes {
		if d.ID != o.ID {
			continue
		}
		for _, x := range d.Types {
			if strings.EqualFold(x, t) {
				return true
			}
		}
		return d.AllCreatureTypes && changelingType(t)
	}
	// No derived entry: the printed face plus intrinsic CDAs, as before. A
	// split card read as the spell being cast has its current half's types
	// only (CR 709.3a).
	if splitReadAsSpell(o, sc) {
		return o.Face().TypeLineHas(t, id) || IntrinsicAllCreatureTypes(o) && changelingType(t)
	}
	return hasTypeID(o, t, id)
}

// hasTypeCtxSub is hasTypeCtx with changelingType(t) precomputed as sub: the
// same reads in the same order, the intrinsic-CDA tails taking sub.
func hasTypeCtxSub(o *state.Object, t string, id cards.TypeWordID, sub bool, sc *SpecContext) bool {
	if sc.ExtraTypes != nil && o.ID == sc.ExtraTypesOwner {
		for _, x := range sc.ExtraTypes {
			if strings.EqualFold(x, t) {
				return true
			}
		}
		return false
	}
	for _, d := range sc.Layers.DerivedTypes {
		if d.ID != o.ID {
			continue
		}
		// derivedTypesFor's first entry for o is authoritative.
		for _, x := range d.Types {
			if strings.EqualFold(x, t) {
				return true
			}
		}
		return sub && d.AllCreatureTypes
	}
	if splitReadAsSpell(o, sc) {
		return o.Face().TypeLineHas(t, id) || sub && IntrinsicAllCreatureTypes(o)
	}
	return hasTypeSub(o, t, id, sub)
}

// changelingType reports whether t is an actual creature subtype. This uses
// a positive authoritative vocabulary rather than treating every type word
// outside an exclusion list as a creature type: Arcane, Alara, and Ajani are
// respectively spell, plane, and planeswalker subtypes, not types Changeling
// grants.
func changelingType(t string) bool { return CreatureTypeWords(t) }

// IntrinsicAllCreatureTypes is the layer-4 CDA basis, before any type-changing
// effects. Bound layer results must use their own marker instead. A face-down
// battlefield permanent has no printed CDA (CR 708.5).
func IntrinsicAllCreatureTypes(o *state.Object) bool {
	if o == nil || o.Face() == nil || (o.FaceDown && o.Zone == state.ZBattlefield) {
		return false
	}
	f := o.Face()
	if f.HasKeywordRef(kwRefChangeling) || f.AllCreatureTypesCDA() {
		return true
	}
	for _, kw := range o.IntrinsicKeywords {
		if cards.KeywordHeadIDOf(kw) == kwChangeling {
			return true
		}
	}
	return false
}

// TypeMatchWords binds a compact layer-4 type list and its semantic all-types
// marker as an authoritative predicate list. The returned slice is read-only.
// A non-nil empty list means "no types", not "read the printed face".
//
// Every reader of the result tests membership (hasTypeCtx, typeWordsHave,
// lockedLayer4Types' filter), so a list that already carries every
// vocabulary word -- the layer-4 walk's list once an all-creature-types
// grant has materialised the vocabulary into it -- is returned as is: the
// appended copy would hold exactly the same words, each a second time, and
// building it on every layer-4 applicability probe of an animated manland
// was a 330-string allocation per probe.
func TypeMatchWords(types []string, all bool) []string {
	if all {
		var have CreatureTypeSet
		if have.AddWords(types) == len(creatureTypeWordList) {
			return types
		}
		return append(append([]string(nil), types...), CreatureTypeWordList()...)
	}
	if types == nil {
		return []string{}
	}
	return types
}

// kwChangeling is Changeling's interned keyword head (cards.InternKeywordHead).
var kwChangeling = cards.InternKeywordHead("Changeling")

type typePredicateCode uint16

const (
	typePredicateSupertype typePredicateCode = iota + 1
	typePredicateNonLand
	typePredicateNonCreature
	typePredicateNonBasic
	typePredicateChosenType
	typePredicateIsNotChosenType
	typePredicateChosenCtrl
	typePredicateExiledWithSource
)

var typePredicateCodes = state.NewStrCodes(
	state.StrEntry[typePredicateCode]{Key: "Legendary", Val: typePredicateSupertype},
	state.StrEntry[typePredicateCode]{Key: "Basic", Val: typePredicateSupertype},
	state.StrEntry[typePredicateCode]{Key: "Snow", Val: typePredicateSupertype},
	state.StrEntry[typePredicateCode]{Key: "nonLand", Val: typePredicateNonLand},
	state.StrEntry[typePredicateCode]{Key: "nonCreature", Val: typePredicateNonCreature},
	state.StrEntry[typePredicateCode]{Key: "nonBasic", Val: typePredicateNonBasic},
	state.StrEntry[typePredicateCode]{Key: "ChosenType", Val: typePredicateChosenType},
	state.StrEntry[typePredicateCode]{Key: "IsNotChosenType", Val: typePredicateIsNotChosenType},
	state.StrEntry[typePredicateCode]{Key: "ChosenCtrl", Val: typePredicateChosenCtrl},
	state.StrEntry[typePredicateCode]{Key: "ExiledWithSource", Val: typePredicateExiledWithSource},
	state.StrEntry[typePredicateCode]{Key: "ExiledWithSourceLKI", Val: typePredicateExiledWithSource},
)
