package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { Register("ReverseTurnOrder", effReverseTurnOrder) }

// effReverseTurnOrder implements AB$ ReverseTurnOrder (Forge's
// ReverseTurnOrderEffect, Aeon Engine's "{T}, Exile Aeon Engine: Reverse the
// game's turn order"): one events.ReverseTurnOrder toggles
// state.Game.ReverseTurnOrder, and the ONE next-player helpers
// (state.Game.NextAlive/AliveFrom) read it, so the turn advance, the
// priority pass order and every APNAP order follow the reversed direction
// with no per-site change. Each activation toggles; two restore the
// original order.
func effReverseTurnOrder(h Host, c *Ctx, sa *cards.SA) {
	h.Emit(events.Event{Kind: events.ReverseTurnOrder, Obj: c.Source})
}
