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
	h           Host
	source      state.ObjID
	controller  state.PlayerID
	amount      int32
	hasLifelink bool
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
	hasLifelink := h.HasKeyword(source, "Lifelink")
	// CR 608.2h: an independently resolving ability whose source is no
	// longer on the battlefield uses that source's last known information.
	// This matters for a granted keyword: Equipment stops applying once its
	// bearer is sacrificed, but the source had lifelink at its last moment on
	// the battlefield. While the source remains there, always prefer its live
	// derived state so detaching the Equipment before resolution removes the
	// rider as it should.
	if c.SourceLifelinkLKIValid {
		hasLifelink = c.SourceLifelinkLKI
	}
	return damageRider{h: h, source: source, controller: c.Controller,
		amount: amount, hasLifelink: hasLifelink}
}

// payLifelinkRider is CR 702.15a's life gain for NON-COMBAT damage: when
// damage from a source with lifelink actually landed, the source's controller
// gains `r.amount` life. Lifelink is not a trigger (CR 702.15a) -- the
// LifeChange rides the damage emit itself, so the gain is visible at the same
// instant the damage is, with no stack, no trigger queue and no ask.
//
// dealt is the amount on the Damage event that actually landed after
// replacement effects. It is zero for prevention, and may differ from the
// proposed amount for a multiplier or other amount-changing replacement.
func payLifelinkRider(r damageRider, dealt int32) {
	if dealt <= 0 || !r.hasLifelink {
		return
	}
	r.h.Emit(events.Event{Kind: events.LifeChange, Player: r.controller,
		Amount: dealt})
}

// emitObjectDamage marks the shared SBA witness only when EmitDamage returns
// positive applied damage. The same returned result gates and prices lifelink,
// so prevention pays neither rider and amount replacement prices both from the
// event that actually landed.
//
// CR 306.8's planeswalker loyalty exchange happens in rules.Engine.emit,
// after this proposed Damage has traversed the same prevention/replacement
// pipeline as every other recipient. EmitDamage still returns the final
// Damage shape so lifelink and deathtouch consume the replaced amount.
func emitObjectDamage(r damageRider, target state.ObjID) {
	h := r.h
	if h.Game().Obj(target) == nil {
		return
	}
	applied := h.EmitDamage(events.Event{Kind: events.Damage, Obj: target, Amount: r.amount})
	dealt := int32(0)
	if applied.Kind == events.Damage {
		dealt = applied.Amount
	}
	if dealt > 0 && applied.Obj != 0 && h.HasKeyword(r.source, "Deathtouch") {
		o := h.Game().Obj(applied.Obj)
		if o != nil && (o.Face() == nil || !o.Face().IsPlaneswalker()) {
			h.Emit(events.Event{Kind: events.CounterChange, Obj: applied.Obj,
				Counter: "Deathtouched", Amount: 1})
		}
	}
	payLifelinkRider(r, dealt)
}

// emitPlayerDamage lands one non-combat Damage event on a player and pays the
// lifelink rider from the amount that survived replacement effects.
func emitPlayerDamage(r damageRider, target state.PlayerID) {
	applied := r.h.EmitDamage(events.Event{Kind: events.Damage, Player: target, Amount: r.amount})
	dealt := int32(0)
	if applied.Kind == events.Damage {
		dealt = applied.Amount
	}
	payLifelinkRider(r, dealt)
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
