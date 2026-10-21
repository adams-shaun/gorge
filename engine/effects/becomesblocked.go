package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { Register("BecomesBlocked", effBecomesBlocked) }

// effBecomesBlocked is api:BecomesBlocked (Trap Runner, Fog Patch, Choking
// Vines, Curtain of Light, Dazzling Beauty): each defined attacking creature
// that is not already blocked becomes blocked (CR 509.1h) with no creature
// blocking it, so it assigns no combat damage unless it has trample
// (CR 702.19e). Its "becomes blocked" triggers fire (CR 509.3c) off the
// BlockedByEffect event; a creature already blocked stays as it is (Forge's
// BecomesBlockedEffect fires nothing for it either).
func effBecomesBlocked(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || !o.IsAttacking || len(o.BlockedBy) != 0 {
			continue
		}
		h.Emit(events.Event{Kind: events.BlockedByEffect, Obj: o.ID, Player: o.Attacking})
		// CR 702.22i: if one member of a band would become blocked due to
		// an effect, the entire band becomes blocked.
		for _, m := range g.BandMates(o.ID) {
			if mo := g.Obj(m); mo != nil && len(mo.BlockedBy) == 0 {
				h.Emit(events.Event{Kind: events.BlockedByEffect, Obj: m, Player: mo.Attacking})
			}
		}
	}
}
