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
		e.emit(takeLandPlayedEvent(e.G, e.etbLandPlayer, &e.etbLandPerm, e.etbLandFrom, id))
	}
}

// takeLandPlayedEvent builds the LandPlayed event for p's play of id and
// spends the may-play permission the play carries.
func takeLandPlayedEvent(g *state.Game, p state.PlayerID, perm *string, from state.Zone, id state.ObjID) events.Event {
	// A land played through a limited untyped may-play static carries the
	// static's permission token (mayPlayKeyLimitReached counts it); every
	// other land play is byte-identical.
	text := ""
	if *perm != "" {
		text = "perm=" + *perm
	}
	*perm = ""
	// A play off the hand, or of a card p does not own, names the land and
	// its origin so events.Apply records it for the filtered
	// PlayerCountPropertyYou$LandsPlayed <filter> read (Spider-Man 2099,
	// Visions of Phyrexia). The ordinary hand play of an owned land stays
	// byte-identical: Obj zero means "from p's hand, owned by p".
	ev := events.Event{Kind: events.LandPlayed, Player: p, Text: text}
	if o := g.Obj(id); o != nil && (from != state.ZHand || o.Owner != p) {
		ev.Obj, ev.From = id, from
	}
	return ev
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
// replacement ask: the CR 616.1 order between competing entry replacements (a
// tapped land beside "lands you control enter untapped"), a lone "you may"
// entry replacement, or the order of its entry-counter replacements (whose
// parked event is the counter, with the move held on the stage). The answer
// need not put the land onto the battlefield, so whoever dispatches it settles
// the play afterwards (settleLandPlayIfDone).
func landEntryOrderParked(choices []replChoice, id state.ObjID) bool {
	for i := range choices {
		ev := &choices[i].ev
		if st := choices[i].stage; choices[i].kind == replChoiceEntryOrder && st != nil {
			ev = &st.move
		}
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZBattlefield {
			return true
		}
	}
	return false
}

// settleLandPlayIfDone is the terminal-path settle: called wherever a land
// play's battlefield entry has finished being processed, it logs the land play
// unless the entry is still parked on an outstanding answer. An entry that
// completed normally already settled through finishLandPlay, so this is a
// no-op there; it fires for the fully-replaced entry, which never reaches the
// battlefield at all, and for an answered entry whose Updated body left an ask
// of its own pending.
func (e *Engine) settleLandPlayIfDone(id state.ObjID) {
	if e.landEntryParked(id) {
		return
	}
	if landEntryOrderParked(e.replChoices, id) {
		// The play is counted as it is made, before the ask (the special
		// action is taken, CR 305.1), but the play stays in flight: the
		// answered entry's MoveZone is what a Mode$ LandPlayed trigger
		// matches (playingLand).
		if e.etbLandPlay && id == e.etbLandObj && !e.etbLandLogged {
			e.emit(takeLandPlayedEvent(e.G, e.etbLandPlayer, &e.etbLandPerm, e.etbLandFrom, id))
			e.etbLandLogged = true
		}
		return
	}
	e.settleLandPlay(id)
}
