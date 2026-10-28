package neutral

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// eachFaceGroup is the index of the first EACH sub-spec of spec the observed
// card matches, or -1. It mirrors engine/effects.EachTypeGroups' first-match
// rule without a game: the neutral v2 rebuild holds only the candidate's
// printed face. The corpus EACH search specs name a type, subtype, colour or
// supertype plus possession/context predicates; the candidate list was already
// filtered by the host's union matcher, and a predicate this helper cannot
// read fails closed, exactly as the engine's textual matcher does for an
// unknown token.
func eachFaceGroup(spec string, f *cards.Face) int {
	rest, ok := strings.CutPrefix(strings.TrimSpace(spec), "EACH ")
	if !ok || f == nil {
		return -1
	}
	for i, sub := range strings.Split(rest, "&") {
		if eachFaceMatches(f, strings.TrimSpace(sub)) {
			return i
		}
	}
	return -1
}

func eachFaceMatches(f *cards.Face, spec string) bool {
	base, rest, _ := strings.Cut(spec, ".")
	if !eachFaceBase(f, base) {
		return false
	}
	for _, p := range strings.Split(rest, "+") {
		if p == "" {
			continue
		}
		if !eachFacePredicate(f, p) {
			return false
		}
	}
	return true
}

func eachFaceBase(f *cards.Face, base string) bool {
	if neg, ok := strings.CutPrefix(base, "non"); ok {
		return !eachFaceWord(f, neg)
	}
	switch base {
	case "Card", "Any":
		return true
	case "Permanent", "Spell":
		// The engine's textual matcher reads these as zone facts; a library
		// card is neither a battlefield permanent nor a stack spell.
		return false
	}
	return eachFaceWord(f, base)
}

func eachFacePredicate(f *cards.Face, p string) bool {
	if neg, ok := strings.CutPrefix(p, "non"); ok {
		return !eachFaceWord(f, neg)
	}
	if name, ok := strings.CutPrefix(p, "named"); ok {
		return f != nil && strings.EqualFold(f.Name, name)
	}
	if name, ok := strings.CutPrefix(p, "notnamed"); ok {
		return f == nil || !strings.EqualFold(f.Name, name)
	}
	switch p {
	case "YouOwn", "YouCtrl", "Owned", "Controlled", "IsRemembered", "Remembered", "Self", "Other":
		// Possession and context predicates: the host's union matcher already
		// selected these candidates, so they are true for every offered card.
		return true
	case "White":
		return f.ColourIdentity()&cards.ColourWhite != 0
	case "Blue":
		return f.ColourIdentity()&cards.ColourBlue != 0
	case "Black":
		return f.ColourIdentity()&cards.ColourBlack != 0
	case "Red":
		return f.ColourIdentity()&cards.ColourRed != 0
	case "Green":
		return f.ColourIdentity()&cards.ColourGreen != 0
	}
	return eachFaceWord(f, p)
}

func eachFaceWord(f *cards.Face, word string) bool {
	return f != nil && f.TypeLineHas(word, 0)
}
