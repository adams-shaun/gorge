package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/chars"
	"github.com/adams-shaun/gorge/engine/state"
)

// underPrintedKeywords is the keyword lines of the cards beneath o's top whose
// own abilities exist: none while o is face down (CR 708.8) or loses its
// printed abilities (CR 613.1f), since then Derived carries no such line
// either. Each line is one instance that card's own trigger already queues.
func (e *Engine) underPrintedKeywords(observer *Engine, o *state.Object) []string {
	if o == nil || len(o.MergedCards) == 0 || observer.printedAbilitiesGone(o) {
		return nil
	}
	return chars.UnderKeywordLines(o)
}

// grantedPileInstances counts the instances of the keyword head a granted walk
// must synthesize for a permanent whose top face does not print it: Derived's
// entries with that head, less one per card beneath the top that prints it
// (that card's own trigger queues those, CR 702.140e).
func (e *Engine) grantedPileInstances(observer *Engine, id state.ObjID, o *state.Object, head string) int {
	return chars.CountKeywordHead(e.Derived(id).Keywords, head) -
		chars.CountKeywordHead(e.underPrintedKeywords(observer, o), head)
}
