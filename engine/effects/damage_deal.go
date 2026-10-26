package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// dealDamageDecider resolves an OptionalDecider$ spec to the seat that makes
// the may election: You/True is the resolving controller, every other spelling
// goes through the shared DefinedSpec grammar (TargetedController,
// ParentTargetedController, ...). A spec that does not yield a player fails
// closed to the caller's Note + mandatory damage.
func dealDamageDecider(h Host, c *Ctx, spec string) (state.PlayerID, bool) {
	if strings.EqualFold(spec, "True") || strings.EqualFold(spec, "You") {
		return c.Controller, true
	}
	resolved := DefinedSpec(h, c, spec)
	if len(resolved) == 0 || !resolved[0].IsPlayer {
		return 0, false
	}
	return resolved[0].Player, true
}

// damageHasRecipient reports whether the damage would reach anything at all:
// a player target, or a permanent still on the battlefield (CR 608.2b). With
// nothing to damage the election is a no-op, so no ask is posed.
func damageHasRecipient(h Host, c *Ctx, sa *cards.SA) bool {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			return true
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			return true
		}
	}
	return false
}

// damageOptionalPrompt names the source, amount and first recipient in the
// election's prompt ("Have Requiem Monolith deal 1 damage to Grizzly Bears?").
func damageOptionalPrompt(h Host, c *Ctx, sa *cards.SA, n int32) string {
	target := "it"
	if ts := Defined(h, c, sa); len(ts) > 0 {
		t := ts[0]
		if t.IsPlayer {
			if g := h.Game(); int(t.Player) < len(g.Players) {
				target = g.Players[t.Player].Name
			}
		} else {
			target = objName(h.Game(), t.Obj)
		}
	}
	return "Have " + objName(h.Game(), c.Source) + " deal " + strconv.Itoa(int(n)) + " damage to " + target + "?"
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
	dp := DealDamageOf(sa)
	if !c.DamageSplitDone {
		// Once per division: an already-filled split has noted.
		noteUnreadParams(h, c, "DealDamage", dp.Unread)
	}
	n := numText(h, c, dp.NumDmg, 0)
	if n < 0 {
		n = 0
	}
	// OptionalDecider$ <spec> (Requiem Monolith: "That creature's controller
	// may have this artifact deal 1 damage to it"): the damage itself is a
	// may election made by the NAMED player, resolved through the same
	// selector grammar Draw's OptionalDecider$ arm uses. Option 0 is today's
	// behaviour (yes), so the bot clamp keeps every bot game byte-identical.
	// An unresolvable spec fails closed: one Note and the damage stays (the
	// pre-ask behaviour); a host that cannot ask (R-9) does the same.
	if dp.OptionalDecider != "" && n > 0 && damageHasRecipient(h, c, sa) {
		if seat, ok := dealDamageDecider(h, c, dp.OptionalDecider); ok {
			d := &decision.Decision{Player: seat, Kind: decision.KChoose, Min: 1, Max: 1,
				Source: c.Source, ResumeKind: "dealdamage_optional", ResumeSA: sa,
				Prompt: damageOptionalPrompt(h, c, sa, n),
				Options: []decision.Option{
					{Index: 0, Kind: "yes", Label: "Yes — deal damage", Player: seat},
					{Index: 1, Kind: "no", Label: "No", Player: seat},
				}}
			if ans, served := AskTape(h, d); served {
				if !answerYes(ans) {
					return
				}
			} else {
				h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
					Text: "DealDamage: no host to ask the OptionalDecider$ election; deals (R-9 stand-in)"})
			}
		} else {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Player: c.Controller,
				Text: "unmodelled DealDamage OptionalDecider$ " + dp.OptionalDecider})
		}
	}
	// ReplaceDyingDefined$ <list> (Chandra, Awakened Inferno's "If a permanent
	// dealt damage this way would die this turn, exile it instead", Wilt in
	// the Heat's Targeted form): registered once the damage batch has landed,
	// over the objects this resolution actually damaged/targeted. The deferred
	// call runs after EndDamageBatch on either path.
	var dying []state.Target
	defer func() { registerReplaceDying(h, c, dp.ReplaceDyingDefined, dying) }()
	// CR 702.15a: damage dealt by a source with lifelink causes that source's
	// controller to gain that much life. Every non-combat emit site below pays
	// the rider through one damageRider; combat has its own rider in
	// engine/rules/combat.go's damage step (the reference shape this class was
	// modelled on).
	rider := newDamageRider(h, c, dp.DamageSource, n)
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
	remember := dp.RememberDamaged
	// DividedAsYouChoose$ N (Fury's "deals 4 damage divided as you choose
	// among any number of target creatures and/or planeswalkers", Forked
	// Bolt): the NAMED TOTAL is divided among the chosen targets, not dealt
	// to each. The player's own division is a real mid-resolution ask: one
	// KChoose option per chosen target, Min == Max == the named total and
	// Repeatable, so the answer is a multiset whose per-target multiplicities
	// are the shares (a target that receives nothing is simply never picked).
	// The decision's own Min/Max/Repeatable wire rules already constrain the
	// answer to exactly the named total over exactly the chosen target list,
	// so no new Validate rule is needed. The deterministic R-9 no-host
	// stand-in distributes one damage at a time,
	// round-robin in the chosen-target order, so the batch total is exactly
	// N. Targets beyond N take nothing, as an unchosen target would.
	divided := false
	var total int32
	if dp.Divided {
		divided = true
		total = numText(h, c, dp.DividedTotal, 0)
		if total < 0 {
			total = 0
		}
	}
	// The chosen-target list both the division ask and the emission walk
	// read, in Defined$ order; the option index of a target is exactly its
	// position here, so the answer's multiplicities land on the right
	// recipient.
	type divTarget struct {
		obj    state.ObjID
		player state.PlayerID
	}
	var divTargets []divTarget
	// split is the allocation the emission walk reads, filled below.
	var split []int32

	if divided {
		var live []state.Target
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				divTargets = append(divTargets, divTarget{player: t.Player})
				live = append(live, t)
				continue
			}
			if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
				divTargets = append(divTargets, divTarget{obj: t.Obj})
				live = append(live, t)
			}
		}
		// CR 601.2d: a division announced while casting is final. Each still-
		// legal target takes exactly its announced share; a target that left
		// or became illegal (CR 608.2b) takes nothing and its share is not
		// redistributed. No ask is posed.
		if !c.DamageSplitDone {
			if shares, ok := AnnouncedDivision(h, c, sa, live); ok {
				split = shares
				c.DamageSplitDone = true
			}
		}
		if !c.DamageSplitDone {
			// Only a division with something to divide is asked; an empty
			// target list or a total of 0 is the silent no-op it has always
			// been. A SINGLE legal target has exactly one legal answer (all of
			// the total), so the ask is filled directly rather than posed --
			// never a decision nobody could answer differently (the
			// strict-supersets rule putCounterChoose and bolster share).
			if len(divTargets) > 1 && total > 0 {
				opts := make([]decision.Option, 0, len(divTargets))
				for i, t := range divTargets {
					o := decision.Option{Index: i, Player: c.Controller}
					if t.obj != 0 {
						o.Kind, o.Obj = "card", t.obj
						if gobj := h.Game().Obj(t.obj); gobj != nil && gobj.Face() != nil {
							o.Label = gobj.Face().Name
						}
					} else {
						o.Kind = "player"
						o.Player = t.player
						if g := h.Game(); int(t.player) < len(g.Players) {
							o.Label = g.Players[t.player].Name
						}
					}
					opts = append(opts, o)
				}
				d := &decision.Decision{Player: c.Controller, Kind: decision.KChoose,
					Min: int(total), Max: int(total), Options: opts, Repeatable: true,
					ResumeKind: "damage_split", ResumeSA: sa, Source: c.Source,
					Prompt: "Assign " + strconv.Itoa(int(total)) + " damage"}
				if ans, ok := AskTape(h, d); ok {
					// Answered in place: the shares; the walk below emits
					// them with the rider built on the way in.
					split = damageSplitAnswer(ans)
				} else {
					// No host (R-9): the deterministic round-robin stand-in.
					split = roundRobinSplit(len(divTargets), total)
				}

			} else if len(divTargets) == 1 && total > 0 {
				// The sole target must receive the whole total: filling the
				// split directly keeps the positional emission loop below
				// honest without posing an unanswerable decision.
				split = []int32{total}
			}
			c.DamageSplitDone = true
		}
	}
	// ExcessSVar$ <name> (CR 120.10): the damage this call deals BEYOND what
	// was lethal to each permanent is published under <name> for the chained
	// SubAbility$ to read (Bottle-Cap Blast's TokenAmount$ Excess, Cramped
	// Vents' LifeAmount$ Excess, Nahiri's Warcrafting's DigNum$ X). The
	// publication is a Ctx.SVars binding -- resolution-scoped scratch, never
	// an event -- which every consumer (effects.Num, EvalCount's SVar$
	// indirection, resolveNumericRHS, CheckSVarHolds) already reads. The
	// condition gates it when present; a condition this build cannot
	// evaluate fails CLOSED (no bind), the conservative direction.
	excessName := dp.ExcessSVar
	excessCond := dp.ExcessSVarCondition
	// bindExcess publishes max(0, dealt - lethal) under the card's name.
	// `lethal` is captured by the caller BEFORE the damage lands -- CR 120.10
	// measures excess against the lethal amount, and marked damage counts
	// toward it, so reading toughness/remaining loyalty after the hit would
	// double-count the damage just dealt.
	bindExcess := func(o *state.Object, lethal, dealt int32) {
		if excessName == "" || o == nil {
			return
		}
		if !excessConditionHolds(h, c, excessCond, o) {
			return
		}
		excess := dealt - lethal
		if excess < 0 {
			excess = 0
		}
		publishSVar(c, excessName, strconv.Itoa(int(excess)))
	}
	// A DamageSource$ spec that resolves to SEVERAL objects makes each of
	// them a separate damager (emitFromEachSource below); a resolved set of
	// one keeps the single-rider path below byte-identical to what it was.
	var multiSources []state.ObjID
	if spec := dp.DamageSource; spec != "" {
		if ts, ok := damageSourceSpecTargets(h, c, spec); ok {
			seen := make(map[state.ObjID]bool)
			for _, t := range ts {
				if t.IsPlayer || t.Obj == 0 {
					continue
				}
				id := resolveSourceObject(h, t.Obj)
				if !seen[id] {
					seen[id] = true
					multiSources = append(multiSources, id)
				}
			}
		}
	}
	// DamageMap$ True (Forge's AbilityFactoryDealDamage "addDamage" into the
	// resolution's damage map, flushed later by DB$ DamageResolve) MARKS this
	// call's damage instead of dealing it. The marks ride Ctx.PendingDamage --
	// the resolution-scratch class of Remembered/SVars, never event-encoded:
	// a replay re-derives them by re-running the same resolution -- and the
	// DamageResolve primitive flushes them as ONE damage batch (an unresolved
	// mark deals nothing, Forge's own semantics for a script that marks
	// without resolving). Per-CALL marking is the reading this build adopts:
	// Forge shares one map on the root ability's resolution, but which root SA
	// a given script's marks share is not verifiable from the scripts here,
	// and per-call marking reproduces every observable the corpus needs (the
	// flush is one batch; a bare flush is silent). A call this shape cannot
	// mark faithfully -- a DividedAsYouChoose distribution or a multi-source
	// DamageSource$ arm, neither of which any DamageMap line in the corpus
	// carries -- falls through to the immediate-deal path unchanged.
	// A DealDamage later in a chain whose root already opened the damage map
	// shares it (Forge SpellAbility.getDamageMap walks to the root SA): Char's
	// "and 2 damage to you" sub carries no DamageMap$ of its own yet lands in
	// the same simultaneous batch as the root's 4. The inheriting call only
	// marks when it has no per-call riders (RememberDamaged$/
	// ReplaceDyingDefined$) the flush could not honour.
	inherits := (len(c.PendingDamage) > 0 || c.DamageMapOpen) && !remember && dp.ReplaceDyingDefined == ""
	if (dp.DamageMap || inherits) && !divided && len(multiSources) < 2 {
		for _, t := range Defined(h, c, sa) {
			if t.IsPlayer {
				c.PendingDamage = append(c.PendingDamage, PendingDamage{rider: rider,
					target: state.Target{Player: t.Player, IsPlayer: true}})
				continue
			}
			if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
				c.PendingDamage = append(c.PendingDamage, PendingDamage{rider: rider,
					target: state.Target{Obj: t.Obj}})
			}
		}
		return
	}
	// One DealDamage call is ONE damage batch (Forge dealDamage): the events
	// this loop emits latch the DamageDealtOnce/DamageDoneOnce triggers
	// together, so a multi-target hit triggers the source's DealtOnce ability
	// once with the batch total and each target's DoneOnce ability once with
	// what that target took. The host opens the batch; emit opens a batch of
	// one for a Damage event that arrives with none open, so a call this
	// primitive never brackets (none today) still latches per event.
	h.BeginDamageBatch()
	if !divided && emitFromEachSource(h, c, sa, dp, multiSources, n, remember, &dying, bindExcess) {
		h.EndDamageBatch()
		return
	}
	if divided {
		// The split is consumed by this one emission walk: a resolution that
		// runs DealDamage with DividedAsYouChoose$ twice (a RepeatEach body or
		// two chained divided subs) must ask again for the second call rather
		// than silently reuse the first call's shares against a different
		// target list. Reset after the walk, on every return path below.
		defer func() {
			c.DamageSplitDone = false
		}()
		for i, t := range divTargets {
			amt := int32(0)
			if i < len(split) {
				amt = split[i]
			}
			if amt <= 0 {
				continue
			}
			r := rider
			r.amount = amt
			if t.obj != 0 {
				if o := h.Game().Obj(t.obj); o != nil {
					lethal, ok := excessLethal(h, o)
					dealt := emitObjectDamage(r, t.obj)
					if ok {
						bindExcess(o, lethal, dealt)
					}
				}
				if remember {
					c.Remembered = append(c.Remembered, state.Target{Obj: t.obj})
					eventRemember(h, c, t.obj)
				}
				dying = append(dying, state.Target{Obj: t.obj})
				continue
			}
			emitPlayerDamage(r, t.player)
			if remember {
				rememberPlayerBothHalves(h, c, t.player)
			}
		}
		h.EndDamageBatch()
		return
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			emitPlayerDamage(rider, t.Player)
			if remember {
				rememberPlayerBothHalves(h, c, t.Player)
			}
			continue
		}
		if o := h.Game().Obj(t.Obj); o != nil && o.Zone == state.ZBattlefield {
			lethal, ok := excessLethal(h, o)
			dealt := emitObjectDamage(rider, t.Obj)
			if ok {
				bindExcess(o, lethal, dealt)
			}
			if remember {
				c.Remembered = append(c.Remembered, state.Target{Obj: t.Obj})
				eventRemember(h, c, t.Obj)
			}
			dying = append(dying, state.Target{Obj: t.Obj})
		}
	}
	h.EndDamageBatch()
}

// emitFromEachSource is DealDamage's multi-source arm: a DamageSource$ spec
// that resolves to SEVERAL objects makes each of them a separate damager over
// the recipients (Missy's "each artifact creature you control deals 1 damage
// to that opponent", Judgment of Alexander's "each commander creature you
// control deals damage equal to its power") -- one rider per source with
// SetDamageSource published around each one's pass (the lifelink rider,
// deathtouch mark, protection and the DamageDone triggers all read THAT
// source), inside the caller's single damage batch, the shape Forge's
// DamageDealEffect per-source loop builds. RelativeTarget$ True re-resolves
// the recipients PER SOURCE through Defined$ anchored on that source (aura
// barbs: each enchantment damages its own controller, each attached Aura the
// creature it is attached to), with a per-source ctx holding only that
// source's identity -- the same shape effEachDamage's per-damager Ctx takes;
// a relative arm whose per-source recipients resolve to nothing contributes
// that source's nothing (fail closed inside, never a guessed pairing). A
// source no longer on the battlefield contributes nothing (Forge reads its
// LKI; the corpus multi-source lines all name live battlefield permanents, so
// the narrower liveness rule is unreachable there and is recorded in the
// ticket report). Returns false when the arm did not run -- fewer than two
// resolvable sources, a shared recipient arm that resolved to nothing, or a
// DividedAsYouChoose resolution (no corpus line combines the two) -- so the
// caller falls through to its single-rider path unchanged.
func emitFromEachSource(h Host, c *Ctx, sa *cards.SA, dp *DealDamageParams, sources []state.ObjID, n int32,
	remember bool, dying *[]state.Target, bindExcess func(o *state.Object, lethal, dealt int32)) bool {
	if len(sources) < 2 {
		return false
	}
	relative := dp.RelativeTarget
	var shared []state.Target
	if !relative {
		shared = Defined(h, c, sa)
		if len(shared) == 0 {
			return false
		}
	}
	emittedAny := false
	for _, src := range sources {
		o := h.Game().Obj(src)
		if o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		pc := NewCtxPtr(src, o.Controller, CtxInit{SVars: c.SVars})
		recips := shared
		if relative {
			recips = Defined(h, pc, sa)
			if len(recips) == 0 {
				continue
			}
		}
		// The per-source rider must not re-resolve the DamageSource$ spec (it
		// would collapse every source onto the first match again): it names
		// no spec, so its source is the per-source ctx's own.
		rider := newDamageRider(h, pc, "", n)
		prev := h.SetDamageSource(rider.source)
		for _, t := range recips {
			if t.IsPlayer {
				emitPlayerDamage(rider, t.Player)
				if remember {
					rememberPlayerBothHalves(h, c, t.Player)
				}
				emittedAny = true
				continue
			}
			if to := h.Game().Obj(t.Obj); to != nil && to.Zone == state.ZBattlefield {
				lethal, ok := excessLethal(h, to)
				dealt := emitObjectDamage(rider, t.Obj)
				if ok {
					bindExcess(to, lethal, dealt)
				}
				if remember {
					c.Remembered = append(c.Remembered, state.Target{Obj: t.Obj})
					eventRemember(h, c, t.Obj)
				}
				*dying = append(*dying, state.Target{Obj: t.Obj})
				emittedAny = true
			}
		}
		h.SetDamageSource(prev)
	}
	return emittedAny
}
