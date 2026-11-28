package rules

import (
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

// grantedPileInstances counts the instances of the keyword head a granted walk
// must synthesize for a permanent whose top face does not print it: Derived's
// entries with that head, less one per card beneath the top that prints it
// (that card's own trigger queues those, CR 702.140e).
func grantedPileInstances(v pileView, id state.ObjID, o *state.Object, head string) int {
	return chars.CountKeywordHead(v.Derived(id).Keywords, head) -
		chars.CountKeywordHead(underPrintedKeywords(v, o), head)
}
