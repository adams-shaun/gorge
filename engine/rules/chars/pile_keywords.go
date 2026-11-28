package chars

import (
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file is the printed half of a merged permanent's abilities (CR
// 702.140e: "A mutated permanent has all abilities of each card and token that
// represents it"): the top face f first, then the faces beneath it in pile
// order. Only abilities span the pile; every other characteristic is the top
// face's. A permanent that is not a pile reads f alone, without allocating.
// A pile that became a copy after its latest merge has no keyword line from
// beneath its top (CR 707.2, state.Object.CopiedSinceMerge): f, the copied
// face, is all it prints.

// PrintedKeywords is the keyword lines o's cards print: f's, then each under
// card's. The result may alias f.Keywords and must not be written.
func PrintedKeywords(o *state.Object, f *cards.Face) []string {
	if o == nil || len(o.MergedCards) == 0 {
		return f.Keywords
	}
	return appendUnderKeywords(slices.Clip(f.Keywords), o)
}

// appendUnderKeywords appends to kw the keyword lines of the cards beneath o's
// top, none once a copy effect covers the merge.
func appendUnderKeywords(kw []string, o *state.Object) []string {
	if o.CopiedSinceMerge() {
		return kw
	}
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil {
			kw = append(kw, uf.Keywords...)
		}
	}
	return kw
}

// UnderKeywordLines is the keyword lines of the cards beneath o's top, none
// for a permanent that is not a pile or that a newer copy effect covers. The
// result is fresh.
func UnderKeywordLines(o *state.Object) []string {
	if o == nil || len(o.MergedCards) == 0 {
		return nil
	}
	return appendUnderKeywords(nil, o)
}

// CountKeywordHead counts the lines of kw whose head is head (any case).
func CountKeywordHead(kw []string, head string) int {
	n := 0
	for _, k := range kw {
		if strings.EqualFold(cards.KeywordHead(k), head) {
			n++
		}
	}
	return n
}

// TakeKeywordLine removes the first line of left equal to k (any case) and
// reports whether there was one. Each printed line is one instance (CR 113.2c),
// so it accounts for exactly one equal derived entry.
func TakeKeywordLine(left []string, k string) ([]string, bool) {
	for i, l := range left {
		if strings.EqualFold(l, k) {
			return slices.Delete(left, i, i+1), true
		}
	}
	return left, false
}

// UnderPrintsHead reports whether a card beneath o's top prints a keyword line
// with the head head (interned as id) that the pile has (UnderKeywordLines).
func UnderPrintsHead(o *state.Object, head string, id cards.KeywordHeadID) bool {
	if o.CopiedSinceMerge() {
		return false
	}
	for i := range o.MergedCards {
		if uf := o.MergedFaceAt(i); uf != nil && uf.KeywordLinesHaveHead(head, id) {
			return true
		}
	}
	return false
}

// PrintedTriggers is the compiled triggers that account for printed derived
// keyword entries: f's, then each under card's whose keyword lines the pile has
// (UnderKeywordLines). The result may alias f.Triggers and must not be written.
func PrintedTriggers(o *state.Object, f *cards.Face) []cards.Trigger {
	if o == nil || len(o.MergedCards) == 0 || o.CopiedSinceMerge() {
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
