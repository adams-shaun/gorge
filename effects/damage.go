package effects

import (
	"strings"

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
	// RememberDamaged$ True makes the resolution remember the objects it
	// damaged, so a SubAbility$ (Incinerate's DB$ Effect reading
	// RememberObjects$ Remembered.Creature) can act on exactly what took the
	// damage. Ctx is threaded by pointer through Resolve, so appending here is
	// visible to the sub-ability without any state write -- the remembered set
	// is per-resolution context, not game state, and replay re-derives it by
	// re-running the same resolution (Task ce1).
	remember := strings.TrimSpace(sa.Params["RememberDamaged"]) != ""
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			h.Emit(events.Event{Kind: events.Damage, Player: t.Player, Amount: n})
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			emitObjectDamage(h, c.Source, t.Obj, n)
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: t.Obj})
			}
		}
	}
}

// emitObjectDamage marks the shared SBA witness only when positive damage from
// a derived-deathtouch source actually increased the recipient's marked
// damage. Host.Emit cannot expose a replacement event, so the state delta is
// the effects-layer observation that protection or prevention did not replace
// the Damage event.
func emitObjectDamage(h Host, source, target state.ObjID, amount int32) {
	o := h.Game().Obj(target)
	if o == nil {
		return
	}
	before := o.Damage
	h.Emit(events.Event{Kind: events.Damage, Obj: target, Amount: amount})
	o = h.Game().Obj(target)
	if amount > 0 && o != nil && o.Damage > before && h.HasKeyword(source, "Deathtouch") {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: target,
			Counter: "Deathtouched", Amount: 1})
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
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				emitObjectDamage(h, c.Source, id, n)
			}
		}
	}
}
