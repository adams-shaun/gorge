package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// finishLandPlay logs a land play only after its identified object actually
// reaches the battlefield. Updated replacement effects fold their MoveZone
// directly through events.Emit, so both the ordinary emit path and those
// replacement continuations call this one finalizer.
func (e *Engine) finishLandPlay(id state.ObjID) {
	if !e.etbLandPlay || id != e.etbLandObj {
		return
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	e.settleLandPlay(id)
}

// settleLandPlay closes the land play's accounting for id and disarms the
// continuation, WITHOUT requiring the land to be on the battlefield. Playing a
// land is a special action that is taken -- and so uses that turn's land play
// (CR 305.1, CR 505.5b) -- the moment it is announced; a replacement effect
// that fully replaces the land's battlefield entry (ReplacementResult$
// Replaced) changes where the land ends up, not whether it was played. Leaving
// the continuation armed instead would both hand the player a second land play
// that turn and let a LATER, unrelated entry of the same object consume the
// stale continuation and log a LandPlayed that belongs to nothing.
func (e *Engine) settleLandPlay(id state.ObjID) {
	if !e.etbLandPlay || id != e.etbLandObj {
		return
	}
	logged := e.etbLandLogged
	e.etbLandPlay, e.etbLandObj, e.etbLandLogged = false, 0, false
	if !logged {
		e.logLandPlay(id)
	}
}

// logLandPlay emits id's LandPlayed event for the armed land play.
func (e *Engine) logLandPlay(id state.ObjID) {
	p := e.etbLandPlayer
	// A land played through a limited untyped may-play static carries the
	// static's permission token (mayPlayKeyLimitReached counts it); every
	// other land play is byte-identical.
	text := ""
	if e.etbLandPerm != "" {
		text = "perm=" + e.etbLandPerm
	}
	e.etbLandPerm = ""
	// A play off the hand, or of a card p does not own, names the land and
	// its origin so events.Apply records it for the filtered
	// PlayerCountPropertyYou$LandsPlayed <filter> read (Spider-Man 2099,
	// Visions of Phyrexia). The ordinary hand play of an owned land stays
	// byte-identical: Obj zero means "from p's hand, owned by p".
	ev := events.Event{Kind: events.LandPlayed, Player: p, Text: text}
	if o := e.G.Obj(id); o != nil && (e.etbLandFrom != state.ZHand || o.Owner != p) {
		ev.Obj, ev.From = id, e.etbLandFrom
	}
	e.emit(ev)
}

// landEntryParked reports whether some engine machinery still holds id's
// battlefield entry and will re-emit it once an outstanding answer arrives: an
// as-enters choice parked by applyETBChoiceReplacement (or the riot/unleash/
// siege fallbacks that park the same way), or a replacement body suspended on a
// mid-resolution ask. While the entry is parked the land play is not settled --
// the entry is still in flight and its own completion reaches finishLandPlay.
func (e *Engine) landEntryParked(id state.ObjID) bool {
	for i := range e.etbQueue {
		if e.etbQueue[i].move.Obj == id {
			return true
		}
	}
	for _, parked := range [...]*events.Event{e.riotMove, e.unleashMove, e.siegeMove} {
		if parked != nil && parked.Obj == id {
			return true
		}
	}
	return false
}

// landEntryOrderParked reports whether id's battlefield entry waits on a
// CR 616.1 order ask between competing entry replacements (a tapped land
// beside "lands you control enter untapped"). The answer composes the entry
// and reaches finishLandPlay.
func (e *Engine) landEntryOrderParked(id state.ObjID) bool {
	for i := range e.replChoices {
		if ev := &e.replChoices[i].ev; ev.Kind == events.MoveZone && ev.Obj == id {
			return true
		}
	}
	return false
}

// settleLandPlayIfDone is the terminal-path settle: called wherever a land
// play's battlefield entry has finished being processed, it logs the land play
// unless the entry is still parked on an outstanding answer. An entry that
// completed normally already settled through finishLandPlay, so this is a
// no-op there; it fires exactly for the fully-replaced entry, which never
// reaches the battlefield at all.
func (e *Engine) settleLandPlayIfDone(id state.ObjID) {
	if e.landEntryParked(id) {
		return
	}
	if e.landEntryOrderParked(id) {
		// The play is counted as it is made, before the order ask (the
		// special action is taken, CR 305.1), but the play stays in flight:
		// the answered entry's MoveZone is what a Mode$ LandPlayed trigger
		// matches (playingLand), and finishLandPlay disarms it then.
		if e.etbLandPlay && id == e.etbLandObj && !e.etbLandLogged {
			e.logLandPlay(id)
			e.etbLandLogged = true
		}
		return
	}
	e.settleLandPlay(id)
}
