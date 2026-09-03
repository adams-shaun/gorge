package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("DealDamage", effDealDamage)
	Register("DamageAll", effDamageAll)
}

// effDealDamage implements "SP$/AB$/DB$ DealDamage" against players and
// permanents. Absorbed from Task 14's stopgap (formerly primitives.go, now
// folded in here): the negative-NumDmg clamp and its default of 0 (not 1) are
// Ruling T14-f, kept verbatim -- events.Apply's Damage case is a plain
// subtraction from Life, so an unclamped negative value would heal instead of
// doing nothing, and TestDealDamageDefaultsMissingNumDmgToZero already locks
// in the zero default.
//
// Folded in on top of that stopgap: a permanent that has already left the
// battlefield (destroyed or sacrificed in response, say) is not a legal
// recipient any more -- the effect does nothing to it rather than marking
// damage on a card sitting in a graveyard. See the Task 18 report for how
// this (and Destroy/ChangeZone/Sacrifice/Counter, which can make exactly that
// happen) interacts with CR 608.2b target rechecking.
func effDealDamage(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumDmg", 0)
	if n < 0 {
		n = 0
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			h.Emit(events.Event{Kind: events.Damage, Player: t.Player, Amount: n})
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			h.Emit(events.Event{Kind: events.Damage, Obj: t.Obj, Amount: n})
		}
	}
}

// effDamageAll is the sweep pattern: iterate the battlefield in seat order,
// filter by ValidCards$ (default "Creature"), emit. Seat order keeps the
// event sequence deterministic.
func effDamageAll(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumDmg", 1)
	if n < 0 {
		n = 0
	}
	spec := sa.Params["ValidCards"]
	if spec == "" {
		spec = "Creature"
	}
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
				h.Emit(events.Event{Kind: events.Damage, Obj: id, Amount: n})
			}
		}
	}
}
