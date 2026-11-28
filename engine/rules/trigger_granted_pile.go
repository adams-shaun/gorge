package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
)

// pileView is the one engine view a granted walk reads both of a merged
// permanent's keyword lists from, so the derived entries and the under cards'
// lines cannot disagree.
type pileView interface {
	Derived(id state.ObjID) Derived
	printedAbilitiesGone(o *state.Object) bool
}

// topPrintedKeywords is the keyword lines of f, o's top face, whose abilities
// exist: none while o is face down (CR 708.8) or loses its printed abilities
// (CR 613.1f). A line that does not exist accounts for no derived entry, so a
// newer grant of the same line is that permanent's own instance (CR 113.2c).
func topPrintedKeywords(v pileView, o *state.Object, f *cards.Face) []string {
	if v.printedAbilitiesGone(o) {
		return nil
	}
	return f.Keywords
}

// topPrintsKeyword reports whether f, o's top face, prints the keyword ref as
// an ability that exists (topPrintedKeywords' gate).
func topPrintsKeyword(v pileView, o *state.Object, f *cards.Face, ref cards.KeywordRef) bool {
	return f.HasKeywordRef(ref) && !v.printedAbilitiesGone(o)
}

// underPrintedKeywords is the keyword lines of the cards beneath o's top whose
// own abilities exist: none while o is face down (CR 708.8) or loses its
// printed abilities (CR 613.1f), since then Derived carries no such line
// either. Each line is one instance that card's own trigger already queues.
func underPrintedKeywords(v pileView, o *state.Object) []string {
	if o == nil || len(o.MergedCards) == 0 || v.printedAbilitiesGone(o) {
		return nil
	}
	return chars.UnderKeywordLines(o)
}

// printedKeywordLines is the printed lines of the permanent that exist, the
// top face's then each under card's. Each owns exactly one equal derived entry,
// the instance its own expansion queues (CR 113.2c, 702.140e). The result is
// fresh: a walk consumes it with chars.TakeKeywordLine.
func printedKeywordLines(v pileView, o *state.Object, f *cards.Face) []string {
	return append(slices.Clone(topPrintedKeywords(v, o, f)), underPrintedKeywords(v, o)...)
}

// grantedPileInstances counts the instances of the keyword head a granted walk
// must synthesize: Derived's entries with that head, less one per printed line
// with it (printedKeywordLines).
func grantedPileInstances(v pileView, id state.ObjID, o *state.Object, f *cards.Face, head string) int {
	return chars.CountKeywordHead(v.Derived(id).Keywords, head) -
		chars.CountKeywordHead(printedKeywordLines(v, o, f), head)
}
