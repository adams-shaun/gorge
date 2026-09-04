package view

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// RedactEvents strips the payload of secret events for every seat but their
// owner, keeping the event's shape so a replay viewer still sees that
// SOMETHING happened (a shuffle, a draw) without learning what. It copies
// rather than mutating: the log is shared across every seat's projection,
// and a live rules.Engine's own log must never be touched by a read path.
//
// Total: a nil evs yields an empty, non-nil slice (supplement §7).
func RedactEvents(evs []events.Event, viewer state.PlayerID) []events.Event {
	out := make([]events.Event, 0, len(evs))
	for _, e := range evs {
		if e.Secret && e.Player != viewer {
			e.IDs = nil
			e.Obj = 0
			e.Text = ""
		}
		out = append(out, e)
	}
	return out
}
