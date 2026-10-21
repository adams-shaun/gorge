package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// heldEntryCheck is one battlefield entry whose trigger check an open zone
// batch holds (Engine.entryHeld): the event as checkTriggers received it (a
// token mint already carries its minted id) and the pre-move LKI emit
// captured. The LKI is never mutated, so a clone shares it.
type heldEntryCheck struct {
	ev  events.Event
	lki *state.Object
}

// cloneHeldEntryChecks copies the held list for a clone: each event by value
// (cloneHeldEvent's discipline), each LKI shared.
func cloneHeldEntryChecks(h []heldEntryCheck) []heldEntryCheck {
	if h == nil {
		return nil
	}
	return append([]heldEntryCheck(nil), h...)
}

// entersBattlefield reports an event that puts a permanent onto the
// battlefield: a MoveZone from another zone, or a token mint.
func entersBattlefield(ev *events.Event) bool {
	switch ev.Kind {
	case events.MoveZone:
		return ev.To == state.ZBattlefield && ev.From != state.ZBattlefield
	case events.TokenCreate, events.CardToken:
		return true
	}
	return false
}

// holdEntryCheck holds ev's trigger check when it is a battlefield entry
// inside an open zone batch (CR 603.6a) and reports whether it did. An
// entry's LKI is never a battlefield object, so the held check needs no
// look-back power/toughness. The LKI is copied off the decision arena,
// since the batch can outlive the decision it was captured in. A token's
// pre-entry object (a CopyToken mint staged outside the battlefield) is no
// last-known information -- a token exists only once it is on the
// battlefield (CR 111.1) -- so the held check reads the entered token, as
// a TokenCreate entry (which carries no LKI) already does.
func (e *Engine) holdEntryCheck(ev *events.Event, lki *state.Object) bool {
	if !e.zoneBatchOpen || e.entryFlushing || !entersBattlefield(ev) {
		return false
	}
	var held *state.Object
	if lki != nil && !lki.IsToken {
		cp := lki.CloneDeep()
		held = &cp
	}
	e.entryHeld = append(e.entryHeld, heldEntryCheck{ev: *ev, lki: held})
	return true
}

// flushHeldEntries runs every held entry check, in emit order, against the
// board as it stands after the batch's last entry (CR 603.6a: "all
// permanents on the battlefield (including the newcomers) are checked").
// closeZoneBatch calls it while the outermost bracket is still open, so a
// Mode$ ChangesZoneAll line still latches once for the batch.
func (e *Engine) flushHeldEntries() {
	held := e.entryHeld
	e.entryHeld = nil
	saved := e.entryFlushing
	e.entryFlushing = true
	for i := range held {
		e.checkTriggers(&held[i].ev, held[i].lki, 0, 0, false)
	}
	e.entryFlushing = saved
}
