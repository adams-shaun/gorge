package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { Register("DrainMana", effDrainMana) }

// effDrainMana is Forge's DrainManaEffect: each defined player loses all
// unspent mana. Power Sink's unless-payment decline and Mana Short are the
// corpus carriers; Defined$ TargetedController names the targeted spell's
// controller, and a plain `Defined$ Player` (Worldpurge) the whole table. The
// pool is emptied through events.ManaClear, one event per player, so replay
// and the persistent-mana tallies stay on their ordinary path. Forge's
// optional DrainMana$ True (the drained mana is added to the activator) and
// RememberDrainedMana$ True are separate shapes kgen refuses.
func effDrainMana(h Host, c *Ctx, sa *cards.SA) {
	for _, p := range definedPlayers(h, c, sa) {
		if int(p) < len(h.Game().Players) {
			h.Emit(events.Event{Kind: events.ManaClear, Player: p, Text: events.ManaDrainText})
		}
	}
}
