package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ownEntryCounterExcluded reports whether an AddCounter replacement whose
// source is the very permanent whose entry counters are being settled must
// stay out of that settlement. CR 614.12: a replacement effect that modifies
// how a permanent enters may come from the permanent itself only if it
// affects that permanent alone, not a general subset that includes it. Dr.
// Joseph M'Benga's "a creature you control" doubler therefore does not double
// the +1/+1 counter it enters with (Deanna Troi's rider); a self-scoped line
// (Mowu's "on CARDNAME", recipient spec Card.Self) still applies. recipient
// is the line's ValidCard$/ValidObject$ spec; recv is the object the counters
// are put on (a placement on any other object is not the entrant's entry).
func ownEntryCounterExcluded(settling, source, recv state.ObjID, recipient string) bool {
	if settling == 0 || source != settling || recv != settling {
		return false
	}
	// Card.Self is the corpus's only self-scoped AddCounter recipient (Mowu).
	return recipient != "Card.Self"
}

// runEntryBody runs an Updated Moved replacement body that the entry fold did
// not absorb (a body with a SubAbility$ rider, Mikey & Don's "enters with an
// additional +1/+1 counter" Effect) with the entrant marked as settling, so
// the counters it puts on the entrant see the same CR 614.12 self-exclusion
// as a folded grant: Michelangelo, Weirdness to 11's "a creature you control"
// line does not add one to the counter it enters with.
func (e *Engine) runEntryBody(ctx *effects.Ctx, stored *events.Event, with *cards.SA) {
	saved := e.entrySettling
	if stored.Kind == events.MoveZone && stored.To == state.ZBattlefield {
		e.entrySettling = stored.Obj
	}
	e.runReplaceWith(ctx, stored.Obj, with, nil)
	e.entrySettling = saved
}
