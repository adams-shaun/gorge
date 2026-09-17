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
	// A multi-target DealDamage is one simultaneous damage event for triggers
	// such as Ob Nixilis's LifeLostAll. The optional hook keeps effects below
	// rules in the package graph; test hosts that do not model trigger queues
	// simply do not implement it.
	if b, ok := h.(interface {
		BeginLifeLossBatch()
		EndLifeLossBatch()
	}); ok {
		b.BeginLifeLossBatch()
		defer b.EndLifeLossBatch()
	}
	// RememberDamaged$ True makes the resolution remember the objects it
	// damaged, so a SubAbility$ (Incinerate's DB$ Effect reading
	// RememberObjects$ Remembered.Creature) can act on exactly what took the
	// damage. Ctx is threaded by pointer through Resolve, so appending here is
	// visible to the sub-ability without any state write -- the remembered set
	// is per-resolution context, not game state, and replay re-derives it by
	// re-running the same resolution (Task ce1).
	remember := strings.TrimSpace(sa.Params["RememberDamaged"]) != ""
	// DividedAsYouChoose$ N (Fury's "deals 4 damage divided as you choose
	// among any number of target creatures and/or planeswalkers", Forked
	// Bolt): the NAMED TOTAL is divided among the chosen targets, not dealt
	// to each. The player's own division choice is an outcome-modelling ask
	// this build does not pose; the deterministic stand-in distributes one
	// damage at a time, round-robin in the chosen-target order, so the last
	// targets of an over-chosen list take nothing and the batch total is
	// exactly N. Targets beyond N take nothing, as an unchosen target would.
	divided := false
	var total int32
	if raw, ok := sa.Params["DividedAsYouChoose"]; ok && strings.TrimSpace(raw) != "" {
		divided = true
		total = Num(h, c, sa, "DividedAsYouChoose", 0)
		if total < 0 {
			total = 0
		}
	}
	// One DealDamage call is ONE damage batch (Forge dealDamage): the events
	// this loop emits latch the DamageDealtOnce/DamageDoneOnce triggers
	// together, so a multi-target hit triggers the source's DealtOnce ability
	// once with the batch total and each target's DoneOnce ability once with
	// what that target took. The host opens the batch; emit opens a batch of
	// one for a Damage event that arrives with none open, so a call this
	// primitive never brackets (none today) still latches per event.
	h.BeginDamageBatch()
	if divided {
		type divTarget struct {
			obj    state.ObjID
			player state.PlayerID
		}
		var ts []divTarget
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				ts = append(ts, divTarget{player: t.Player})
				continue
			}
			if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
				ts = append(ts, divTarget{obj: t.Obj})
			}
		}
		dealt := make(map[divTarget]int32, len(ts))
		for i := int32(0); i < total && len(ts) > 0; i++ {
			t := ts[i%int32(len(ts))]
			dealt[t]++
		}
		for _, t := range ts {
			amt := dealt[t]
			if amt <= 0 {
				continue
			}
			r := rider
			r.amount = amt
			if t.obj != 0 {
				emitObjectDamage(r, t.obj)
				if remember {
					c.Remembered = append(c.Remembered, state.Target{Obj: t.obj})
					eventRemember(h, c, t.obj)
				}
				continue
			}
			emitPlayerDamage(r, t.player)
		}
		h.EndDamageBatch()
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			emitPlayerDamage(rider, t.Player)
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			emitObjectDamage(rider, t.Obj)
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: t.Obj})
				eventRemember(h, c, t.Obj)
			}
		}
	}
	h.EndDamageBatch()
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
	// CR 608.2h: a source that left while this resolution waited uses LKI.
	// The own-source fields cover the independently resolving ability's own
	// permanent; DamageSourceLKI covers a distinct named source such as
	// TriggeredCard or Remembered. Live battlefield state always wins, so a
	// detached lifelink grant is not retained after a source stays in play.
	live := false
	if o := h.Game().Obj(source); o != nil && o.Zone == state.ZBattlefield {
		live = true
	}
	controller := c.Controller
	if !live {
		if source == own && c.SourceLifelinkLKIValid {
			hasLifelink = c.SourceLifelinkLKI
			controller = c.SourceControllerLKI
			if !c.SourceControllerLKIValid {
				controller = c.Controller
			}
		} else if lki, ok := c.DamageSourceLKI[source]; ok {
			hasLifelink = lki.Lifelink
			controller = lki.Controller
		} else if o := h.Game().Obj(source); o != nil {
			// A non-permanent source (for example a spell on the stack) still
			// has a live controller even though it is not a battlefield object.
			controller = o.Controller
		}
	} else if o := h.Game().Obj(source); o != nil {
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
// Damage shape so lifelink and deathtouch consume the replaced amount. A
// creature-planeswalker still needs its Counter "creature" tag set on the
// proposed event (events.Apply cannot import rules' layer engine, so it
// cannot discover the animation itself) so it gets marked damage as well as
// loyalty loss; printed creatures and plain planeswalkers are unaffected by
// the tag.
func emitObjectDamage(r damageRider, target state.ObjID) {
	h := r.h
	o := h.Game().Obj(target)
	if o == nil {
		return
	}
	ev := events.Event{Kind: events.Damage, Obj: target, Amount: r.amount}
	if h.IsCreature(target) && o.Face() != nil && o.Face().IsPlaneswalker() && !o.Face().IsCreature() {
		ev.Counter = "creature"
	}
	applied := h.EmitDamage(ev)
	dealt := int32(0)
	if applied.Kind == events.Damage {
		dealt = applied.Amount
	}
	if dealt > 0 && applied.Obj != 0 && h.HasKeyword(r.source, "Deathtouch") {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: applied.Obj,
			Counter: "Deathtouched", Amount: 1})
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
	// DamageAll is one simultaneous damage event even though its individual
	// hits are serialized in the log. Keep its complete permanent-and-player
	// pass inside the boundary so LifeLostAll observes the affected group once.
	if b, ok := h.(interface {
		BeginLifeLossBatch()
		EndLifeLossBatch()
	}); ok {
		b.BeginLifeLossBatch()
		defer b.EndLifeLossBatch()
	}
	spec := strings.TrimSpace(sa.Params["ValidCards"])
	g := h.Game()
	rider := newDamageRider(h, c, sa, n)
	prev := h.SetDamageSource(rider.source)
	defer h.SetDamageSource(prev)
	// One DamageAll call is ONE damage batch, exactly like DealDamage's
	// (see effDealDamage): every creature and player it hits latches
	// together.
	h.BeginDamageBatch()
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
	h.EndDamageBatch()
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
