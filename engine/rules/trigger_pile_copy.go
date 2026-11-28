package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// underWardReplaced reports whether t, a trigger line of the face at pile
// index merged (0 is the top) of o, is the ward of a card beneath the top that
// a copy effect newer than the merge replaced (state.Object.CopiedSinceMerge).
// Ward is a keyword ability (CR 702.21a), so it goes with the under card's
// keyword line, which the derived list then lacks (chars.UnderKeywordLines):
// the copied face's ward and a granted one are the pile's only instances
// (CR 707.2, 702.140e).
func underWardReplaced(o *state.Object, merged int, t *cards.Trigger) bool {
	return merged != 0 && o.CopiedSinceMerge() && isWardTrigger(t)
}
