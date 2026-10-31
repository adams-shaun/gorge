package event

import (
	"fmt"
)

// The two timeout entries (E1-8) let the host tear down or carry an
// abandoned event without the clock ever reaching a replay: the timer only
// APPENDS entries, and these two apply purely from the log state.

// applyLobbyTimeout cancels an event whose lobby window closed with no
// EventCreated: it is the only path to PhaseCancelled. It must land while
// the event is still uncooked.
func (e *Event) applyLobbyTimeout(en *Entry) error {
	if e.cooked {
		return fmt.Errorf("event: LobbyTimeout: event already created")
	}
	e.phase = PhaseCancelled
	return nil
}

// applyIdleTimeout marks one seat the host acted for because its human
// went quiet with a decision owed. It carries only the seat: the actions
// the host took for it follow as their own ordinary entries (Picked with
// Auto, DeckSaved/Ready, GameEnded with Head "conceded").
func (e *Event) applyIdleTimeout(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: IdleTimeout: event not created")
	}
	switch e.phase {
	case PhaseDraft, PhaseBuild:
	default:
		if !e.inRound() {
			return fmt.Errorf("event: IdleTimeout: illegal in phase %s", e.phase)
		}
	}
	if en.Seat < 0 || en.Seat >= e.NumSeats() {
		return fmt.Errorf("event: IdleTimeout: seat %d out of range", en.Seat)
	}
	return nil
}
