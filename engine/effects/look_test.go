package effects

import (
	"github.com/adams-shaun/gorge/engine/events"
)

// The class of Gitaxian Probe's Look$ leak: every
// looker-scoped effect records its private look through effects' emitLook —
// one Secret Note per looker, Player = that looker. This file pins that
// contract across ALL of them, table-driven, so the next look-shaped effect
// that bypasses the helper fails here.

// secretLookNotes returns the Secret look Notes (emitLook's shape).
func secretLookNotes(log []events.Event) []events.Event {
	var out []events.Event
	for _, e := range log {
		if e.Kind == events.Note && e.Secret {
			out = append(out, e)
		}
	}
	return out
}
