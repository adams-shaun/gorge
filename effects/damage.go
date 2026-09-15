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
//
// CR 609.7a provenance: the damage's source is DamageSource$ when the script
// names one, the resolving source otherwise, and SetDamageSource makes that
// object the one rules' protection check and DamageDone triggers read for
// every Damage event this loop emits (the rider and the engine override
// cannot disagree -- both come from the one resolved rider source).
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
	rider := newDamageRider(h, c, sa, n)
	prev := h.SetDamageSource(rider.source)
	defer h.SetDamageSource(prev)
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

// resolveSourceObject unwraps one object id to the permanent a damage rider
// gates on: an ability stack object (minted by AbilityPush/TriggerPush, Card
// == nil, no Face) reads nothing off itself, so the keywords lifelink and
// deathtouch belong to the SOURCE permanent that pushed it (CR 607.2). A
// spell's own stack object IS the card and is read as-is. Same rule
// rules.Engine.protectionSource applies (Task 15 fix round 1, Critical C2).
func resolveSourceObject(h Host, id state.ObjID) state.ObjID {
	if o := h.Game().Obj(id); o != nil && o.Ability != nil && o.Source != 0 {
		return o.Source
	}
	return id
}

// newDamageRider builds the rider for one resolving DealDamage/DamageAll.
// The source is DamageSource$ when the script names one -- resolved through
// the same Defined resolver every other object reference uses (Scourge of
// Valkas' TriggeredCard, Kiku's Shadow's Targeted), never guessed at the
// chosen targets when Defined does not recognise the spec -- and the
// resolving source otherwise. A DamageSource$ the resolver cannot model
// (EffectSource's LKI provenance, Imprinted, a Valid-card spec) keeps the
// resolving source: today's behaviour, and the conservative direction --
// damage from the resolving spell/ability's source, never damage silently
// attributed to a target. Resolving here, once per resolution, keeps the
// next rider honest: every keyword read below reads r.source, already
// resolved, and SetDamageSource publishes the same object.
func newDamageRider(h Host, c *Ctx, sa *cards.SA, amount int32) damageRider {
	own := resolveSourceObject(h, c.Source)
	source := own
	if spec := strings.TrimSpace(sa.Params["DamageSource"]); spec != "" {
		if ts, ok := definedSpec(h, c, spec); ok {
			for _, t := range ts {
				if t.IsPlayer || t.Obj == 0 {
					continue
				}
				source = resolveSourceObject(h, t.Obj)
				break
			}
		}
	}
	hasLifelink := h.HasKeyword(source, "Lifelink")
	// CR 608.2h: an independently resolving ability whose source is no
	// longer on the battlefield uses that source's last known information.
	// This matters for a granted keyword: Equipment stops applying once its
	// bearer is sacrificed, but the source had lifelink at its last moment on
	// the battlefield. While the source remains there, always prefer its live
	// derived state so detaching the Equipment before resolution removes the
	// rider as it should. The snapshot is about the ability's OWN source --
	// a DamageSource$ naming a different object must not inherit it, because
	// the departure capture never looked at that object.
	if c.SourceLifelinkLKIValid && source == own {
		hasLifelink = c.SourceLifelinkLKI
	}
	// Lifelink belongs to the resolved damaging object, not to the spell or
	// ability's controller. Object.Controller survives a zone change, so it
	// is also the available LKI controller if the named source left before
	// this independently resolving effect dealt damage.
	controller := c.Controller
	if o := h.Game().Obj(source); o != nil {
		controller = o.Controller
	}
	return damageRider{h: h, source: source, controller: controller,
		amount: amount, hasLifelink: hasLifelink}
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
	if !landed || r.amount <= 0 || !r.hasLifelink {
		return
	}
	r.h.Emit(events.Event{Kind: events.LifeChange, Player: r.controller,
		Amount: r.amount})
}

// emitObjectDamage emits one non-combat Damage event against a battlefield
// permanent and pays the lifelink rider for it. The event is the whole
// walker exchange too (CR 306.8/120.3c): events.Apply's Damage case converts
// a planeswalker's damage into loyalty loss -- and still marks a
// creature-planeswalker's damage -- in the same fold every Damage event
// takes, so protection, prevention and DamageDone triggers see walker damage
// exactly like every other damage, and no emitter can forget the conversion.
//
// Host.Emit cannot expose a replacement event, so the state delta is the
// effects-layer observation that protection or prevention did not replace
// the Damage event -- the same observation gates the lifelink rider
// (payLifelinkRider), so a prevented hit pays no deathtouch marker and no
// life. A planeswalker's damage lands as loyalty counters (never marked
// damage, CR 120.3c), so its delta is the walker's own loyalty counter; a
// creature's is the marked-damage total; a creature-planeswalker moves both
// and either delta reports the hit.
func emitObjectDamage(r damageRider, target state.ObjID) {
	h := r.h
	o := h.Game().Obj(target)
	if o == nil {
		return
	}
	walker := o.Face() != nil && o.Face().IsPlaneswalker()
	var beforeDamage int32
	var beforeLoyalty int32
	if walker {
		beforeLoyalty = o.Counter("LOYALTY")
	} else {
		beforeDamage = o.Damage
	}
	ev := events.Event{Kind: events.Damage, Obj: target, Amount: r.amount}
	// events.Apply cannot import rules' layer engine. Carry the current
	// creature result on the Damage event so an animated planeswalker gets
	// marked damage as well as loyalty loss; printed creatures remain a
	// backwards-compatible fallback for direct event users.
	if h.IsCreature(target) && o.Face() != nil && o.Face().IsPlaneswalker() && !o.Face().IsCreature() {
		ev.Counter = "creature"
	}
	h.Emit(ev)
	o = h.Game().Obj(target)
	landed := false
	if o != nil && r.amount > 0 {
		if walker {
			landed = o.Counter("LOYALTY") < beforeLoyalty
		} else {
			landed = o.Damage > beforeDamage
		}
	}
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

// effDamageAll is the sweep pattern: when ValidCards$ is present, iterate the
// battlefield in seat order, filter and emit; then run the independent
// ValidPlayers$ arm (Pestilence, Valakut Exploration, Earthquake). An absent
// ValidCards$ means no object half at all -- Valakut's player-only sweep must
// not inherit a synthetic Creature default. Seat order keeps the event
// sequence deterministic. DamageSource$ applies to the player arm too (the
// lifelink rider pays for player damage from a named source), through the same
// rider.
func effDamageAll(h Host, c *Ctx, sa *cards.SA) {
	n := Num(h, c, sa, "NumDmg", 1)
	if n < 0 {
		n = 0
	}
	spec := strings.TrimSpace(sa.Params["ValidCards"])
	g := h.Game()
	rider := newDamageRider(h, c, sa, n)
	prev := h.SetDamageSource(rider.source)
	defer h.SetDamageSource(prev)
	if spec != "" {
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if MatchesSpecCtx(g, spec, id, c.SpecContext(c.Controller)) {
					emitObjectDamage(rider, id)
				}
			}
		}
	}
	for _, p := range validPlayers(h, c, sa.Params["ValidPlayers"]) {
		emitPlayerDamage(rider, p)
	}
}

// validPlayers resolves a DamageAll ValidPlayers$ spec to the players the
// sweep damages, in a deterministic order (selector order first, then seat
// order). A spec Defined recognises as an object reference resolves through
// the same resolver every other reference uses -- ValidPlayers$ Targeted
// (players among the chosen targets), Remembered -- while anything else is
// a PREDICATE over every living player through MatchesPlayerSpec (Player,
// Player.Opponent, Opponent, You). A spec neither resolves (the
// OppNonTriggeredTarget / FlippedTails singletons) stays unsupported and
// damages no player: fail closed, never a guess about who takes the sweep.
func validPlayers(h Host, c *Ctx, spec string) []state.PlayerID {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}
	g := h.Game()
	appendPlayer := func(out []state.PlayerID, seen map[state.PlayerID]bool,
		p state.PlayerID) []state.PlayerID {
		if p < 0 || int(p) >= len(g.Players) || g.Players[p].Lost || seen[p] {
			return out
		}
		seen[p] = true
		return append(out, p)
	}
	if spec == "TargetedController" {
		// The controller of the chosen object target (1 corpus line); the
		// object-target analogue of ValidPlayers$ Targeted.
		if len(c.Targets) == 0 {
			return nil
		}
		return []state.PlayerID{PlayerOf(h, c, c.Targets[0])}
	}
	if ts, ok := definedSpec(h, c, spec); ok {
		var out []state.PlayerID
		seen := make(map[state.PlayerID]bool)
		for _, t := range ts {
			if t.IsPlayer {
				out = appendPlayer(out, seen, t.Player)
			}
		}
		return out
	}
	// OppNonTriggeredTarget (Kediss, Emberclaw Familiar, 9 corpus lines):
	// every opponent except the player the triggering event targeted --
	// "deals that much damage to each OTHER opponent". Absent a player
	// TriggerTarget the whole opponent set matches, the same reading the
	// spec's name gives a trigger with no player referent.
	if spec == "OppNonTriggeredTarget" {
		exclude, has := state.PlayerID(0), false
		if c.TriggerTarget.IsPlayer {
			exclude, has = c.TriggerTarget.Player, true
		}
		var out []state.PlayerID
		for _, p := range g.AliveFrom(0) {
			if p != c.Controller && (!has || p != exclude) {
				out = append(out, p)
			}
		}
		return out
	}
	var out []state.PlayerID
	for _, p := range g.AliveFrom(0) {
		if MatchesPlayerSpec(g, spec, p, c.Controller) {
			out = append(out, p)
		}
	}
	return out
}
