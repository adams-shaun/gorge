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
	// CR 702.15a: damage dealt by a source with lifelink causes that source's
	// controller to gain that much life. Every non-combat emit site below pays
	// the rider through one damageRider; combat has its own rider in
	// rules/combat.go's damage step (the reference shape this class was
	// modelled on).
	rider := newDamageRider(h, c, n)
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
			emitPlayerDamage(rider, t.Player)
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			emitObjectDamage(rider, t.Obj)
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: t.Obj})
			}
		}
	}
}

// damageRider bundles the facts every non-combat damage emit site shares: the
// host, the dealing source, that source's controller, and the amount. Passing
// it through one struct is what makes the lifelink rider impossible to forget
// at a new site -- a new emitter takes a rider, and payLifelinkRider runs for
// it unless the emitter can prove the damage did not land.
type damageRider struct {
	h          Host
	source     state.ObjID
	controller state.PlayerID
	amount     int32
}

// newDamageRider builds the rider for one resolving DealDamage/DamageAll.
// The source is resolved through the same rule rules.Engine.protectionSource
// applies (Task 15 fix round 1, Critical C2): an ability stack object is
// minted by AbilityPush/TriggerPush with Card == nil and no Face, so Derived
// -- and with it every derived keyword, printed or granted -- reads nothing
// off the wrapper itself; the keywords a damage rider gates on (lifelink,
// deathtouch) belong to the SOURCE permanent that pushed it (CR 607.2). A
// spell's own stack object IS the card and is read as-is. Resolving here,
// once per resolution, keeps the next rider honest: every keyword read below
// reads r.source, already resolved.
func newDamageRider(h Host, c *Ctx, amount int32) damageRider {
	source := c.Source
	if o := h.Game().Obj(source); o != nil && o.Ability != nil && o.Source != 0 {
		source = o.Source
	}
	return damageRider{h: h, source: source, controller: c.Controller, amount: amount}
}

// payLifelinkRider is CR 702.15a's life gain for NON-COMBAT damage: when
// damage from a source with lifelink actually landed, the source's controller
// gains `r.amount` life. Lifelink is not a trigger (CR 702.15a) -- the
// LifeChange rides the damage emit itself, so the gain is visible at the same
// instant the damage is, with no stack, no trigger queue and no ask.
//
// landed must be false when the damage event did not land: prevention (CR
// 702.16d, protection) replaces a Damage event with a Note, and a prevented
// hit gains nothing. Every caller that can observe the replacement reports it
// (emitObjectDamage's state-delta check); the player arm cannot be prevented
// in this build (protection arms objects only, rules/engine.go's emit checks
// ev.Obj != 0), so it reports landed for any positive amount.
func payLifelinkRider(r damageRider, landed bool) {
	if !landed || r.amount <= 0 || !r.h.HasKeyword(r.source, "Lifelink") {
		return
	}
	r.h.Emit(events.Event{Kind: events.LifeChange, Player: r.controller,
		Amount: r.amount})
}

// emitObjectDamage marks the shared SBA witness only when positive damage from
// a derived-deathtouch source actually increased the recipient's marked
// damage. Host.Emit cannot expose a replacement event, so the state delta is
// the effects-layer observation that protection or prevention did not replace
// the Damage event -- the same observation gates the lifelink rider
// (payLifelinkRider), so a prevented hit pays no deathtouch marker and no
// life.
//
// CR 306.8: damage dealt to a planeswalker permanent removes that many
// loyalty counters instead of being marked as damage, so a walker target
// takes a LOYALTY CounterChange and never a Damage event here. Both spell/
// ability damage paths route through this one helper (effDealDamage's object
// arm and effDamageAll); combat damage cannot reach it -- this build's
// attackers declare player defenders only, and a walker can neither attack
// nor block -- so spell/ability damage is the whole walker-damage surface.
// The exchange is one-directional by design and recorded in AGENTS.md:
// prevention and destruction-replacement effects key on Damage events, so
// they do not see walker damage (prevention vs a walker is unimplemented,
// conservative and correct for now). RememberDamaged$ on the caller still
// captures the walker, so an Incinerate-style "can't be regenerated" Effect
// sub-ability still finds what took the damage.
func emitObjectDamage(r damageRider, target state.ObjID) {
	h := r.h
	o := h.Game().Obj(target)
	if o == nil {
		return
	}
	if f := o.Face(); f != nil && f.IsPlaneswalker() {
		// A walker's damage removes loyalty counters (CR 306.8) but is still
		// damage dealt -- the lifelink rider pays for it too.
		landed := false
		if r.amount != 0 {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: target,
				Counter: "LOYALTY", Amount: -r.amount})
			landed = r.amount > 0
		}
		payLifelinkRider(r, landed)
		return
	}
	before := o.Damage
	h.Emit(events.Event{Kind: events.Damage, Obj: target, Amount: r.amount})
	o = h.Game().Obj(target)
	landed := r.amount > 0 && o != nil && o.Damage > before
	if landed && h.HasKeyword(r.source, "Deathtouch") {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: target,
			Counter: "Deathtouched", Amount: 1})
	}
	payLifelinkRider(r, landed)
}

// emitPlayerDamage lands one non-combat Damage event on a player and pays the
// lifelink rider for it. The player arm has no prevention path in this build
// (see payLifelinkRider), so any positive amount landed.
func emitPlayerDamage(r damageRider, target state.PlayerID) {
	r.h.Emit(events.Event{Kind: events.Damage, Player: target, Amount: r.amount})
	payLifelinkRider(r, r.amount > 0)
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
	rider := newDamageRider(h, c, n)
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
				emitObjectDamage(rider, id)
			}
		}
	}
}
