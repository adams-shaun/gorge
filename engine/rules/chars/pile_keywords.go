package chars

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is the printed half of a merged permanent's abilities (CR
// 702.140e: "A mutated permanent has all abilities of each card and token that
// represents it"): the top face f first, then the faces beneath it in pile
// order. Only abilities span the pile; every other characteristic is the top
// face's. A permanent that is not a pile reads f alone, without allocating.

// PrintedKeywords is the keyword lines o's cards print: f's, then each under
// card's. The result may alias f.Keywords and must not be written.
func PrintedKeywords(o *state.Object, f *cards.Face) []string {
	if o == nil || len(o.MergedCards) == 0 {
		return f.Keywords
	}
	return appendUnderKeywords(slices.Clip(f.Keywords), o)
}

// appendUnderKeywords appends to kw the keyword lines of the cards beneath o's
// top.
func appendUnderKeywords(kw []string, o *state.Object) []string {
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil {
			kw = append(kw, uf.Keywords...)
		}
	}
	return kw
}

// PrintsKeyword reports whether f or a card beneath o's top prints the keyword
// r names.
func PrintsKeyword(o *state.Object, f *cards.Face, r cards.KeywordRef) bool {
	if f.HasKeywordRef(r) {
		return true
	}
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil && uf.HasKeywordRef(r) {
			return true
		}
	}
	return false
}

// UnderPrintsHead reports whether a card beneath o's top prints a keyword line
// with the head head (interned as id).
func UnderPrintsHead(o *state.Object, head string, id cards.KeywordHeadID) bool {
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil && uf.KeywordLinesHaveHead(head, id) {
			return true
		}
	}
	return false
}

// PrintedTriggers is f's compiled triggers, then each under card's. The result
// may alias f.Triggers and must not be written.
func PrintedTriggers(o *state.Object, f *cards.Face) []cards.Trigger {
	if o == nil || len(o.MergedCards) == 0 {
		return f.Triggers
	}
	out := slices.Clip(f.Triggers)
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil {
			out = append(out, uf.Triggers...)
		}
	}
	return out
}

// printedPrefixLen is the length of the printed prefix BaseKeywords puts first.
func printedPrefixLen(o *state.Object, f *cards.Face, faceDown bool) int {
	if faceDown {
		return len(f.Keywords)
	}
	return len(PrintedKeywords(o, f))
}
