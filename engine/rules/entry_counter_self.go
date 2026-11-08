package rules

import "github.com/adams-shaun/gorge/engine/state"

// ownEntryCounterExcluded reports whether an AddCounter replacement whose
// source is the very permanent whose entry counters are being settled must
// stay out of that settlement. CR 614.12: a replacement effect that modifies
// how a permanent enters may come from the permanent itself only if it
// affects that permanent alone, not a general subset that includes it. Dr.
// Joseph M'Benga's "a creature you control" doubler therefore does not double
// the +1/+1 counter it enters with (Deanna Troi's rider); a self-scoped line
// (Mowu's "on CARDNAME", recipient spec Card.Self) still applies. recipient
// is the line's ValidCard$/ValidObject$ spec.
func ownEntryCounterExcluded(settling, source state.ObjID, recipient string) bool {
	if settling == 0 || source != settling {
		return false
	}
	// Card.Self is the corpus's only self-scoped AddCounter recipient (Mowu).
	return recipient != "Card.Self"
}
