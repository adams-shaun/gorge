package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// effRingTemptsYou performs one "the Ring tempts you" action (CR 701.54a):
// the tempted player's count rises by one and a creature they control
// becomes (or stays) their Ring-bearer. The designation and the count fold
// through events.Apply's RingTemptsYou case, so a replay derives both from
// the log alone.
//
// The bearer choice is the documented R-9 stand-in for the player choice CR
// 701.54a names: if the player already controls their existing Ring-bearer
// (still on the battlefield under them), it keeps the designation — which is
// also what CR 701.54a implies, since the designation persists "until
// another creature becomes your Ring-bearer"; otherwise the first eligible
// creature in the player's battlefield zone order is designated (the
// deterministic ordered list, never a map range). A player who controls no
// creature emits the event with Obj 0: the temptation still counts and the
// trigger still fires (CR 701.54d — "even if some were impossible").
func effRingTemptsYou(h Host, c *Ctx, sa *cards.SA) {
	// Measured corpus: none of the 49 raw RingTemptsYou SA lines carries
	// Defined$/ValidTgts$, so the tempted player is always the resolving
	// controller. A Defined$-driven path would be untested dead code whose
	// PlayerOf behaviour on a non-player object reference is undefined for
	// this shape -- it stays out deliberately.
	p := c.Controller
	g := h.Game()
	bearer := state.ObjID(0)
	if cur := g.Players[p].RingBearer; cur != 0 {
		if o := g.Obj(cur); o != nil && o.Zone == state.ZBattlefield && o.Controller == p {
			bearer = cur
		}
	}
	if bearer == 0 {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if h.IsCreature(id) {
				bearer = id
				break
			}
		}
	}
	h.Emit(events.Event{
		Kind:   events.RingTemptsYou,
		Player: p,
		Obj:    bearer,
		Amount: g.Players[p].RingTempted + 1,
	})
}
