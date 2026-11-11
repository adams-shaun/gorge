package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { Register("OpenAttraction", effOpenAttraction) }

// effOpenAttraction resolves DB$ OpenAttraction (CR 701.51a): a player may
// open an Attraction only in a game in which they are playing with an
// Attraction deck. Gorge has no Attraction deck (no zone, no deck import), so
// the action always takes the no-deck branch and records a no-op Note, the
// same degrade as notePlanechaseNoDeck. This function is the seam a future
// Attraction-deck tier branches from.
func effOpenAttraction(h Host, c *Ctx, _ *cards.SA) {
	const text = "open an Attraction (no Attraction deck)"
	if c != nil && c.Source != 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: text})
		return
	}
	h.Emit(events.Event{Kind: events.Note, Text: text})
}
