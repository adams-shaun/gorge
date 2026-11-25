package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { Register("TakeInitiative", effTakeInitiative) }

// effTakeInitiative implements DB$/SP$/AB$ TakeInitiative (CR 726, "The
// Initiative"): the resolving player takes the initiative designation.
//
// CR 726.3: only one player can have the initiative at a time; taking it
// moves the single designation, so the transition is recorded as one
// events.InitiativeChange (folded in engine/events/apply.go), exactly the
// api:BecomeMonarch shape.
//
// CR 726.2's inherent ability -- "Whenever a player takes the initiative,
// that player ventures into Undercity" -- is a real TRIGGERED ability, so it
// is NOT part of this effect's resolution. Each take queues it through
// Host.QueueInitiativeVenture; the venture then goes on the stack, and only
// its resolution enters Undercity and asks the room ability (CR 309.4c).
// This applies even when the taker already had the initiative (CR 726.5:
// the venture triggers but no second designation is created -- the single
// holder assignment already models that).
//
// CR 726.4 (the holder leaving the game hands the initiative to the active
// player) is not implemented, the same scope api:BecomeMonarch left for
// CR 725.4.
func effTakeInitiative(h Host, c *Ctx, sa *cards.SA) {
	players := definedPlayers(h, c, sa)
	if len(players) == 0 {
		return
	}
	for _, p := range players {
		if p < 0 || int(p) >= len(h.Game().Players) {
			continue
		}
		h.Emit(events.Event{Kind: events.InitiativeChange, Player: p})
		h.QueueInitiativeVenture(p)
	}
}
