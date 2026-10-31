package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
)

// emit is the engine's single mutation entry point. Task 20 inserts
// replacement effects ahead of logging and trigger discovery behind it:
// applyReplacements runs first (skipped, via applyingReplacement, while
// another replacement's own resolution is already in flight, which is what
// keeps a self-replacing event from looping), and if it substitutes the
// event, the original is discarded rather than logged -- the substitute's
// own emit (from inside effects.Resolve) already logged whatever needed
// logging. Otherwise the event is logged and folded into state exactly as
// before, and checkTriggers then looks for anything it just made true.
func deferAttackTriggerCheck(kind events.Kind, declarationActive bool) bool {
	return kind == events.DeclareAttackers && declarationActive
}

func triggerCheckEvent(stored events.Event, tokenMintWant state.ObjID) events.Event {
	if stored.Kind == events.TokenCreate || stored.Kind == events.CardToken {
		stored.Obj = tokenMintWant
	}
	return stored
}

// deferredPushKind reports the mint events pushCast and pushActivation emit
// under deferCastTrigger: a spell's PutOnStack and an activated ability's five
// mint kinds. Their cast/activate triggers fire only after payment.
func deferredPushKind(k events.Kind) bool {
	switch k {
	case events.PutOnStack, events.AbilityPush, events.KeywordAbilityPush,
		events.GrantAbilityPush, events.GainedAbilityPush, events.DelayedPush:
		return true
	}
	return false
}

func (e *Engine) emit(ev events.Event) events.Event {
	// E1 (emit-action.md): objects minted during this event are compared
	// against this count to raise the pool gates (the mint scan below).
	objsBefore := len(e.G.Objs)
	if suppressSuspectedEvent(e, ev) {
		return ev
	} // CR 702.157
	if suppressPhaseEvent(e, ev) {
		return ev
	} // CR 702.26
	if bookkeepingKind(ev.Kind) && !e.applyingReplacement {
		e.emitBookkeeping(&ev)
		return ev
	}
	if ev.Kind == events.EndTurn {
		// CR 724.1a: the triggered abilities that triggered before the
		// end-the-turn process began and are still waiting to be put onto
		// the stack cease to exist. Dropped HERE, before the fold exiles the
		// stack (724.1b), so an ability that triggers during the process
		// survives into the cleanup step (724.1f; finishEndTurn).
		e.pendingTriggers, e.orderedTriggers = nil, 0
		e.endTurnRequested = true
	}
	// Task 15 (CR 702.16d/e): prevent protected Damage and Attach events before
	// replacement, reporting prevention as a Note. Damage uses the published
	// source (or e.damaging); zero means no source. This precedes planeswalker
	// loyalty conversion, and CantPreventDamage bypasses the prevention gate.
	// A non-positive Damage event removes marked damage; it is not damage, so
	// protection never stops it.
	if ev.Kind == events.Damage && ev.Amount > 0 {
		src := e.inFlightDamageSource()
		protected := ev.Obj != 0 && e.protectedFrom(ev.Obj, src)
		if ev.Obj == 0 && int(ev.Player) < len(e.G.Players) {
			protected = e.playerProtectedFrom(ev.Player, src)
		}
		if src != 0 && protected && !e.cantPreventDamage(src, ev.Obj) {
			// The stored Note carries Amount for DamagePreventedOnce triggers.
			return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
				Amount: ev.Amount, Text: "prevented: protection"})
		}
	}
	// CR 701.3a/b: an Attach that would fasten the on-battlefield attachment
	// to the bearer it is already attached to does nothing (no event at
	// all), and one that would fasten it to a bearer the attachment SBAs
	// (engine/rules/attach.go attachmentSBAs) would at once undo is refused
	// with one Note, the object keeping its current bearer. The refusal runs
	// BEFORE the protection guard so a refused attach never detaches.
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 {
		if refused, note := e.attachRefused(ev.Obj, ev.IDs[0]); refused {
			if !note {
				return events.Event{}
			}
			return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Text: "cannot attach: illegal bearer"})
		}
	}
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 &&
		e.protectedFrom(ev.IDs[0], ev.Obj) {
		// Defence in depth: current targeting rules make this unreachable, but
		// future protection types, pre-attached Auras or Attach paths without a
		// targeting step may reach it. Keep the guard central; tests must prove
		// a reachable game state rather than calling emit directly.
		return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Text: "cannot attach: protected"})
	}
	// CR 701.3b: an attach that goes ahead from a DIFFERENT former bearer
	// publishes the Unattached first (the Mode$ Unattached family: Captain's
	// Hook, Grafted Exoskeleton, Grafted Wargear, Stitcher's Graft) so a
	// refused attach never detaches. Moved here from engine/effects'
	// emitAttach, which only effects-side attach paths reach: every Attach
	// path (Living Weapon, token AttachedTo$, aura entry) funnels through
	// this emit. A fresh attach (AttachedTo == 0) or the same-bearer no-op
	// swallowed above emits none.
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 {
		if o := e.G.Obj(ev.Obj); o != nil && o.AttachedTo != 0 && o.AttachedTo != ev.IDs[0] {
			e.emit(events.Event{Kind: events.Unattached, Obj: ev.Obj,
				IDs:  []state.ObjID{o.AttachedTo},
				Text: "reattached to a new permanent"})
		}
	}
	if ev.Kind == events.Attach && ev.Obj != 0 && ev.Text == "attach to player" {
		if attaching := e.G.Obj(ev.Obj); attaching != nil && e.isAura(attaching) &&
			int(ev.Player) < len(e.G.Players) && e.playerProtectedFrom(ev.Player, ev.Obj) {
			return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
				Text: "cannot attach: protected"})
		}
	}
	// A CantPutCounter restriction swallows a counter placement outright
	// (task cantputcounter1): the placement never happens, so neither the
	// event nor any AddCounter replacement of it may run. This gate was
	// hoisted here, out of applyReplacementsDispatch, so it runs EVEN while a
	// replacement effect's own ReplaceWith$ body is resolving
	// (applyingReplacement): the guard that stops a replacement from
	// re-matching its own event must not also swallow the prohibition, or a
	// counter an "enters with N counters" body places slips past Solemnity
	// and friends (task addcounter1/2). Looking at it before the replacement
	// pass is harmless: applyReplacementsDispatch used to run it at its own
	// top, before any match was collected.
	//
	// Only a POSITIVE placement of a real counter is subject to the
	// restriction: a removal (Amount <= 0) is not a placement at all, and the
	// engine's own status markers (regeneration's Shield, the Deathtouched
	// mark) are not counters -- the same state.InternalCounterMarker exclusion the
	// AddCounter matcher keeps, so a "counters can't be put on it" static
	// cannot stop a regeneration shield or a removal.
	if (ev.Kind == events.CounterChange || ev.Kind == events.PlayerCounterChange) &&
		ev.Amount > 0 && !state.InternalCounterMarker(ev.Counter) {
		if e.PutCounterBlocked(ev.Counter, ev.Obj, ev.Player, ev.Kind == events.PlayerCounterChange) {
			return events.Event{}
		}
	}
	// CR 702.34a and its family: a keyword-cast spell leaving the stack is
	// exiled wherever it would go, before any replacement sees the move.
	e.keywordExileDeparture(&ev)
	if prepareEmitReplacement(e, &ev, e.applyingReplacement, e.replAction, e.replReplaced) {
		return events.Event{}
	}
	// CR 303.4g: a non-cast Aura with nothing it can legally enchant never
	// enters -- it stays in its zone, ahead of every replacement, staging,
	// fold and trigger (engine/rules/aura_entry.go).
	if (ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && auraEntryGate(e, &ev)) ||
		(ev.Kind == events.TokenCreate && auraTokenGate(e, &ev)) {
		return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
			Text: "aura entry withheld (CR 303.4g)"}
	}
	// Entry-counter staging (task agent-20260923T084704Z-b2386c25): an entry
	// whose characteristic counters compete under non-commuting AddCounter
	// replacements stages behind CR 616.1's order choice, BEFORE anything
	// folds -- no observer (an ETB trigger, an SBA, a chapter queue) may see
	// the un-replaced entry while the ask is outstanding. The pre-pass sits
	// here, before the replacement dispatch and the whole fold tail, so a
	// staged entry returns the same handled shape a parked replacement does
	// and the re-drive after the answer runs the ordinary emit exactly once.
	// A completed stage returns false and falls through: the fold below
	// consumes it (engine/rules/entry_counters.go).
	if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield &&
		e.entryCounterOrderParks(ev) {
		return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
			Text: "entry awaiting counter-replacement-order choice"}
	}
	// A replacement body's counter placement is a NEW event, not the event
	// whose replacement body is resolving. Give it its own AddCounter pass;
	// the rewritten event itself is folded below without another pass.
	if !e.applyingReplacement || ((ev.Kind == events.CounterChange || ev.Kind == events.PlayerCounterChange) && !e.counterReplacementFold) {
		replaced, handled := e.applyReplacements(ev)
		if handled {
			return replaced
		}
		// Not replaced, but possibly REWRITTEN in place (a DamageDone
		// ReplaceEffect body changed the amount): the returned event is what
		// gets logged, not the emit caller's copy.
		ev = replaced
	} else if ev.Kind == events.Draw && len(e.replDrawChain) > 0 {
		// CR 616.1e/614.5: a Draw a draw-replacement body emits is the
		// modified draw of the one that replacement replaced. It gets its
		// own replacement pass -- Thought Reflection's two cards still meet
		// Notion Thief, Asmodeus or a second Blood Scrivener -- with every
		// replacement already applied to this draw's lineage excluded, this
		// body's own included, so the chain terminates (CR 614.5: a
		// replacement effect gets one opportunity per event). The applied
		// set is empty outside a draw-replacement body, so an unrelated
		// draw's emit is untouched.
		savedExclude := e.replExclude
		e.replExclude = e.replDrawChain
		replaced, handled := e.applyReplacements(ev)
		e.replExclude = savedExclude
		if handled {
			return replaced
		}
		ev = replaced
	} else if e.redirectRecheck(ev) {
		replaced, handled := e.applyRedirectReplacements(ev)
		if handled {
			return replaced
		}
		ev = replaced
	}
	// Token replacement effects must settle before entry staging: they may
	// remove the mint or rewrite its script. Final token plans re-enter here
	// under applyingReplacement, so they skip rematching but still stage each
	// finalized mint before its TokenCreate/CardToken fold.
	if (ev.Kind == events.TokenCreate || ev.Kind == events.CardToken) &&
		e.entryCounterOrderParks(ev) {
		return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
			Text: "entry awaiting counter-replacement-order choice"}
	}
	// CountersRemain is a departure property of the battlefield object. Tag the
	// final, replacement-adjusted MoveZone so events.Apply and replay preserve
	// the counters in the same fold. Hand and library remain explicit reset
	// destinations per the static's rules text.
	if ev.Kind == events.MoveZone && ev.To != state.ZHand && ev.To != state.ZLibrary {
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield && e.countersRemainApplies(ev.Obj) {
			ev.Counter = events.MarkCountersRemainMove(ev.Counter)
		}
	}
	// CR 603.10a: "a creature died this turn" reads the permanent as it last
	// existed on the battlefield. Tag the final departure when its layer-4
	// creature status differs from the layer-free default Apply can read, so
	// the folded ZoneEntry.WasCreature is the last known information (an
	// animated Soulstone Sanctuary that dies was a creature).
	if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To != state.ZBattlefield {
		ev.Step = 0
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield &&
			e.IsCreature(ev.Obj) != events.DepartureDefaultCreature(o) {
			ev.Step = events.DepartureCreatureFlip
		}
	}
	// DamageDone may rewrite the recipient through ReplaceEvent, while an
	// ordinary hit still needs its initial recipient form classified. Do this
	// after the complete replacement pass so both paths share one rule.
	if ev.Kind == events.Damage {
		e.recomputeInfectMarker(&ev)
		e.recomputeWitherMarker(&ev)
		e.markDamageNotPlaneswalker(&ev)
		e.recomputeWalkerCreatureMarker(&ev)
	}
	// CR 306.8's planeswalker loyalty exchange (and CR 120.3e's exception for
	// a permanent that is also a creature) is folded directly into this
	// Damage event by events.Apply below -- AddCounter("LOYALTY", ...) runs
	// in the same Apply call that would otherwise mark damage, so replay
	// derives it from the one logged Damage event and no separate
	// CounterChange is ever emitted for it.
	// CR 603.10a: a leaves-the-battlefield trigger's eligibility is read
	// against the game state as it was immediately before the event. The SBA
	// paths (engine/rules/sba.go) park that board in triggerBefore around their
	// batches; an effect destroy, a cost sacrifice or any other departure that
	// funnels through THIS emit used to emit with triggerBefore nil, so the
	// trigger's own grant gate was read only AFTER the departure had already
	// switched it off (Relic Vial's IsPresent$-Cleric AddTrigger$ grant dying
	// with the only Cleric it names). Park the pre-departure board around the
	// fold here -- the same immutable snapshot the SBA discipline shares, the
	// same split pass checkTriggers already consumes -- so every departure
	// route looks back the same way. The guard keeps an outer batch's parked
	// board winning: every departure inside an SBA batch or a parked
	// replacement window still observes that ONE shared board, and the
	// deferred restore keeps triggerBefore nil at intent boundaries (clone.go
	// deliberately does not copy it). The snapshot is taken AFTER the
	// replacement pass settled, so it is the board the folded move actually
	// departs from; recursion inside this emit (the Role sweep above, a
	// replacement body's own move) snapshots its own departure before this
	// window opens or reuses this board like any batch member.
	e.noteBatchDeparture(ev)
	if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield &&
		ev.To != state.ZBattlefield && e.triggerBefore == nil {
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield {
			saved := e.triggerBefore
			own := e.snapshotTriggerBoard()
			e.triggerBefore = own
			defer e.closeTriggerWindow(own, saved)
		}
	}
	// Open a singleton batch before the pre-fold snapshot, not after Apply.
	onlyEventBatch := ev.Kind == events.Damage && !e.damageBatchOpen
	if onlyEventBatch {
		e.openDamageBatch()
	}
	e.excessDamageBaseline = captureExcessBaseline(e.excessDamageBaseline, ev, e)
	// LKI (CR 603.10 "look back in time") is captured HERE, before
	// events.Emit runs Apply and mutates the object -- a zone-change trigger
	// needs the object exactly as it was a moment ago (its counters, tapped
	// state, damage, controller, zone), not the reset state Move leaves it
	// in. See effects.Ctx.LKI.
	var lki *state.Object
	var lkiPower, lkiToughness int32
	var lkiPTValid bool
	switch ev.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack, events.ControlChange:
		if o := e.G.Obj(ev.Obj); o != nil {
			lki = e.cloneDeepArenaObject(o)
			if o.Zone == state.ZBattlefield && o.Face() != nil {
				lkiPower, lkiToughness = e.Power(o.ID), e.Toughness(o.ID)
				lkiPTValid = true
			}
		}
	case events.DoorUnlock:
		// CR 709.5: Mode$ FullyUnlock (engine/rules/trigmatch/room.go) must tell a
		// real locked->unlocked transition from a repeated DoorUnlock on an
		// already-unlocked room (the latter no game action produces, but a
		// direct emit can). Apply flips Unlocked before this event's triggers
		// are matched, so the pre-fold flag has to ride the LKI snapshot.
		if o := e.G.Obj(ev.Obj); o != nil {
			cp := o.CloneDeep()
			lki = e.arenaObject(&cp)
		}
	case events.CounterChange:
		// Vanishing's last-counter trigger must distinguish a real removal
		// from a redundant decrement at zero. Keep the pre-fold TIME count
		// alongside the existing Suspend zero-crossing check below.
		if ev.Amount < 0 && ev.Counter == "TIME" {
			if o := e.G.Obj(ev.Obj); o != nil {
				cp := o.CloneDeep()
				lki = &cp
			}
		}
	}
	departingSource, departingSourceLifelink, departingSourceController := e.captureSourceLifelinkLKI(ev)
	timeBefore := int32(0)
	if lki != nil && ev.Kind == events.CounterChange {
		timeBefore = lki.Counter("TIME")
	}
	// CR 310.11 (battle-defeated): the defeat feed below reads the defense
	// count BEFORE the move fold -- Apply's Move clears o.Counters as the
	// object leaves the battlefield, so a post-move read would call every
	// exiled battle defeated (a blink of a healthy Siege would pose the
	// transformed-cast offer too).
	defenseBefore := int32(0)
	if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
		if o := e.G.Obj(ev.Obj); o != nil {
			defenseBefore = o.Counter("DEFENSE")
		}
		// CR 608.2b/h departure boundary: this is the last moment a departing
		// target of the resolving chain still carries the counters the
		// look-back reads, so refresh the chain's Ctx.TargetCountersLKI HERE --
		// after the replacement pass settled the final move, before events.Apply
		// clears the counters. A chained effect that added or removed counters
		// earlier in the same resolution must be read as it was immediately
		// before the zone change, not as the resolution-start snapshot
		// (effects.Resolve's entry capture) recorded it.
		e.snapshotDepartingTargetCounters(ev.Obj)
	}
	stackLen := len(e.G.Stack)
	// Record only the final event after replacement selection. The object
	// snapshot must precede Apply, and unknown adder provenance is not a
	// match for either You or Player. The engine's own status markers (the
	// regeneration Shield, the Deathtouched lethal mark) ride a positive
	// CounterChange but are not counters a player PUT -- and a resolving
	// deathtouch damage ability has an actionCause, so without the marker
	// exclusion its emitted mark would be attributed to that controller and
	// make Count$CountersAddedThisTurn <Any> You Creature spuriously true.
	// The same exclusion engine/rules/replacement.go's doubler gate keeps.
	if ev.Kind == events.CounterChange && ev.Amount > 0 && !state.InternalCounterMarker(ev.Counter) {
		if actor, ok := e.inFlightCounterAdder(); ok {
			if o := e.G.Obj(ev.Obj); o != nil {
				e.counterAddsThisTurn = append(e.counterAddsThisTurn, counterAddedThisTurn{
					actor: actor, kind: ev.Counter, amount: ev.Amount, object: o.CloneDeep(),
				})
			}
		}
	}
	var abilityMintWant state.ObjID
	switch ev.Kind {
	case events.AbilityPush, events.KeywordAbilityPush, events.GrantAbilityPush, events.GainedAbilityPush:
		abilityMintWant = e.G.NextID
	}
	var tokenMintWant state.ObjID
	if ev.Kind == events.TokenCreate || ev.Kind == events.CardToken || ev.Kind == events.CopyToken {
		tokenMintWant = e.G.NextID
	}
	var stackCopyMintWant state.ObjID
	if ev.Kind == events.StackCopy && e.stackCopyMintSink != nil {
		stackCopyMintWant = e.G.NextID
	}
	wasTapped := false
	if ev.Kind == events.Untap {
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield {
			wasTapped = o.Tapped
		}
	}
	stored := ev
	absorbedEntry := e.foldEntryMove(&stored, true)
	e.expireClonesOnEvent(stored, wasTapped)
	e.expireUntilTargetedUntaps(stored, wasTapped)
	if stored.Kind == events.MoveZone { // CR 303.4f: a non-cast Aura enters attached to its chosen bearer.
		settleAuraEntry(e, &stored)
	}
	// CR 310.10: every Battle whose recorded protector has just left the game
	// gets a fresh living opponent as its protector. PlayerLost is the one
	// funnel every departure passes through (life, poison, an empty-library
	// draw, a concession), and events.Emit has already marked the seat Lost
	// by the time this returns, so protectorOpponents reads the departure.
	// The re-derive emits a Choose "protector" event (it never poses a
	// decision), so it is safe to run here even mid-resolution.
	if stored.Kind == events.PlayerLost {
		e.rechooseDepartedBattleProtector(stored.Player)
	}
	e.publishTokenEntry(stored, tokenMintWant)
	if (stored.Kind == events.TokenCreate || stored.Kind == events.CardToken) && tokenMintWant != 0 {
		// Other permanents' "enters tapped / with a counter" replacements
		// apply to a token's entry too (engine/rules/token_entry_replacements.go).
		e.applyTokenEntryUpdates(tokenMintWant, absorbedEntry)
	}
	// E1 (emit-action.md): any object minted during this event (a token, a
	// named-card or permanent copy, an ability wrapper) is a card the genesis
	// census never saw, so raise this match's pool gates on it BEFORE the
	// refreshes below. The raise is monotone and a pure function of the log,
	// so a replay reaches the same gates at every position; the objsBefore
	// compare covers every mint path without a per-kind list.
	for id := objsBefore + 1; id <= len(e.G.Objs); id++ {
		if o := e.G.Obj(state.ObjID(id)); o != nil && o.Card != nil {
			e.notePoolCard(o.Card)
		}
	}
	e.recordTurnLedgers(stored, abilityMintWant)
	if stackCopyMintWant != 0 && e.G.Obj(stackCopyMintWant) != nil {
		*e.stackCopyMintSink = append(*e.stackCopyMintSink, stackCopyMintWant)
	}
	if ev.Kind == events.CounterChange && ev.Amount < 0 && ev.Counter == "TIME" && timeBefore > 0 {
		// CR 702.62a/b (counterchoice1): the LAST time counter leaving a
		// suspended card by ANY route — the upkeep tick or a Clockspinning/
		// Amy-Pond-style removal mid-resolution — queues CR 702.62a's may-cast
		// offer; startSuspendedCast drains the queue at the next step(). The
		// ONE home replaces the upkeep tick's own append (which was the only
		// emitter before): a counter removed mid-resolution used to strand a
		// zero-TIME card in exile forever, because the tick skips a card
		// already at zero and nothing else ever re-offered the cast.
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone == state.ZExile &&
			(o.CastFlags&state.FlagSuspend != 0 || o.SuspendGranted) && o.Counter("TIME") == 0 {
			e.suspendedCasts = append(e.suspendedCasts, ev.Obj)
		}
	}
	// CR 310.11 (battle-defeated): a Battle leaving the battlefield for exile
	// with no defense counters was exiled by the defeat SBA (engine/rules/sba.go's
	// battleZeroDefense), so its owner's transformed-cast offer is queued
	// here -- the ONE home every route to that exile shares, so a replayed
	// game re-derives the queue from the same event. defenseBefore (read
	// above, pre-fold) is what makes "no defense counters" precise; FaceIdx 0
	// plus a second face are what the "cast it transformed" half needs.
	if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To == state.ZExile &&
		defenseBefore == 0 {
		if o := e.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().IsBattle() &&
			o.FaceIdx == 0 && len(o.Card.Faces) > 1 && o.Counter("DEFENSE") == 0 {
			e.defeatedCasts = append(e.defeatedCasts, ev.Obj)
		}
	}
	// CR 702.90b (kw:Infect): the counters/poison an infect source's damage
	// is dealt in the form of are placed HERE, as real events emitted
	// through this same emit -- so the repl:AddCounter class (a Winding
	// Constrictor doubler, a CantPutCounter lock) and trig:CounterAdded see
	// the placement exactly like any other, and engine/rules/sba.go's CR 704.5b
	// ten-poison loss reads a real PlayerCounterChange fold. The marker was
	// set by the emitter (engine/rules/combat.go, engine/rules/cast.go,
	// engine/rules/resolution.go, engine/effects/damage.go) after it checked HasKeyword on
	// the source; this conversion classifies the form off the event that
	// actually landed (the replaced/prevented hit never reaches here -- a
	// prevention is a Note), and the recipient-creature half of the marker
	// is the emitter's layer-accurate classification the fold reuses.
	if stored.Kind == events.Damage && stored.Amount > 0 &&
		(stored.Counter == "infect" || stored.Counter == "infect+creature") {
		e.convertInfectDamage(stored)
	}
	if stored.Kind == events.Damage && stored.Amount > 0 && stored.Counter == "wither+creature" {
		e.convertWitherDamage(stored)
	}
	// Game-long damage-by-source provenance (the_fallen, diseased_vermin):
	// every landed Damage event appends a DamageProvenance fact so the
	// wasDealtDamageThisGameBy player qualifier and the
	// wasDealtDamageByThisGame object predicate can answer Forge's game-long
	// record. This lives HERE, on the one post-fold tail, because every
	// emitter (engine/effects/damage.go's riders, engine/rules/combat.go's combat batch,
	// engine/rules/cast.go, engine/rules/resolution.go and the cleanup negatives) funnels
	// through emit -- no emitter file has to change. It reads `stored`, the
	// APPLIED event, so post-protection/post-replacement/post-redirect it
	// names the real recipient and the amount that actually landed; a
	// prevented hit is a Note and never reaches here, and a cleanup negative
	// is excluded by the Amount > 0 gate (exactly like the infect/wither
	// conversions above). The source is the same published override / e.damaging
	// reader emit's own protection guard uses; a zero source (no recorded
	// provenance) emits nothing rather than minting a false (0, recipient)
	// fact. The recipient is stored.Obj when nonzero (an object) else
	// stored.Player (a seat), encoded PlayerRef-style so seat 0 is
	// distinguishable from "no recipient".
	if stored.Kind == events.Damage && stored.Amount > 0 {
		if src := e.inFlightDamageSource(); src != 0 {
			var recipient state.ObjID
			if stored.Obj != 0 {
				recipient = stored.Obj
			} else {
				recipient = state.PlayerRef(stored.Player)
			}
			recordDamageProvenance(e.emit, e.G.Obj(src), e.G.Obj(stored.Obj), recipient, stored.Amount, e.combatDamaging, e.objColors(e.G.Obj(src)), e.EffectiveTypes())
		}
	}
	e.noteTurnsTaken(&stored)
	// The per-turn combat-damage ledger expires with the turn (CR 514.2's
	// "this turn" window): a TurnChange begins a fresh turn, so every hit
	// captured during the turn that just ended is no longer "this turn".
	if stored.Kind == events.TurnChange {
		e.combatHitsThisTurn = nil
		e.counterAddsThisTurn = nil
		e.activationsThisTurn = nil
		e.crimeSeatsThisTurn = 0
		e.bendSeatsThisTurn = [64]uint8{}
		e.noncombatDamagedSeatsLast, e.noncombatDamagedSeats = e.noncombatDamagedSeats, 0
	}
	e.loop.observeFrom(&stored, e.damaging, len(e.G.Objs))
	// setname.go: keep the layer-3 rename table the filter tier reads in step
	// with the board. Gated so a match with no SetName$ carrier pays one
	// branch.
	if e.setNameInPool {
		e.refreshRenames()
	}
	// layer4types.go: keep the layer-4 derived-type table the filter tier reads
	// in step with the board. Gated so a match with no type-changing carrier
	// pays one branch.
	if e.layer4InPool {
		e.refreshDerivedTypes()
	}
	if ev.Kind == events.StackCopy && len(e.G.Stack) > stackLen {
		copyID := e.G.Stack[len(e.G.Stack)-1]
		// StackCopy inherits the flat targets in events.Apply; preserve the
		// cast-time declaration split too. Current legality cannot reconstruct
		// which half owned an inherited target after the board has changed.
		if stages, ok := e.fuseTargets[ev.Obj]; ok && len(ev.IDs) == 0 {
			if e.fuseTargets == nil {
				e.fuseTargets = make(map[state.ObjID][][]state.Target)
			}
			cp := make([][]state.Target, len(stages))
			for i, targets := range stages {
				cp[i] = append([]state.Target(nil), targets...)
			}
			e.fuseTargets[copyID] = cp
		}
		// CR 707.10: "If an effect of the copy refers to objects used to pay
		// its costs, it uses the objects used to pay the costs of the
		// original spell or ability" (a copied Fling deals the ORIGINAL's
		// sacrificed creature's power). The copy takes its own copy of each
		// cost-paid record the original's resolution Ctx reads (Forge
		// SpellAbility.copy shares the paid hash the same way).
		// CR 707.10c: a copy copies the chain's announced targets too. A copy
		// whose SubAbility$ chain it announces (copyChainDeclarations) takes
		// the original's per-link record under its own id; an electing copy
		// overwrites each link as its copy_targets answers arrive
		// (finishCopyTargets), a non-electing one resolves with them as they
		// stand.
		if copyChainRoot(e, e.G.Obj(copyID)) != nil {
			for line, ts := range e.castSubTargets[ev.Obj] {
				e.recordCastSub(copyID, line, append([]state.Target{}, ts...))
			}
		}
		inheritCopyRecord(e.sacrificedLKI, ev.Obj, copyID)
		inheritCopyRecord(e.castExiled, ev.Obj, copyID)
		inheritCopyRecord(e.castRevealed, ev.Obj, copyID)
		inheritCopyRecord(e.castDiscarded, ev.Obj, copyID)
		inheritCopyRecord(e.castCounterPut, ev.Obj, copyID)
		inheritCopyRecord(e.castTapped, ev.Obj, copyID)
		inheritCopyRecord(e.castReturned, ev.Obj, copyID)
		if tc, ok := e.triggerContexts[ev.Obj]; ok {
			e.triggerContexts[copyID] = tc
		}
		if ef, ok := e.triggerEffectFrames[ev.Obj]; ok {
			e.triggerEffectFrames[copyID] = ef
		}
		if line, ok := e.triggerLines[ev.Obj]; ok {
			if e.triggerLines == nil {
				e.triggerLines = make(map[state.ObjID]cards.Trigger)
			}
			e.triggerLines[copyID] = line
			if svars, ok := e.triggerLineSVars[ev.Obj]; ok {
				if e.triggerLineSVars == nil {
					e.triggerLineSVars = make(map[state.ObjID]map[string]string)
				}
				e.triggerLineSVars[copyID] = svars
			}
		}
		if lki, ok := e.triggerLKI[ev.Obj]; ok {
			if e.triggerLKI == nil {
				e.triggerLKI = make(map[state.ObjID]triggerObjectLKI)
			}
			if lki.object != nil {
				cp := lki.object.CloneDeep()
				lki.object = &cp
			}
			e.triggerLKI[copyID] = lki
		}
		if link, ok := e.sourceLifelinkLKI[ev.Obj]; ok {
			if e.sourceLifelinkLKI == nil {
				e.sourceLifelinkLKI = make(map[state.ObjID]bool)
			}
			e.sourceLifelinkLKI[copyID] = link
		}
		if controller, ok := e.sourceControllerLKI[ev.Obj]; ok {
			if e.sourceControllerLKI == nil {
				e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
			}
			e.sourceControllerLKI[copyID] = controller
		}
		if snap, ok := e.sourceCharLKI[ev.Obj]; ok {
			e.setSourceCharLKI(copyID, snap)
		}
		if lki := e.damageSourceLKI[ev.Obj]; lki != nil {
			if e.damageSourceLKI == nil {
				e.damageSourceLKI = make(map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI)
			}
			e.damageSourceLKI[copyID] = cloneDamageSourceLKI(lki)
		}
	}
	if ev.Kind == events.MoveZone {
		// CR 400.7: an object that changes zones is a new object with no
		// memory of having become tapped this turn.
		delete(e.tappedTurn, ev.Obj)
		// An exploited object's as-sacrificed snapshot is only meaningful
		// while the object is where the exploit left it; a later move (the
		// graveyard card exiled) retires it rather than letting the map grow
		// for the rest of the game.
		delete(e.exploitedLKI, ev.Obj)
	}
	if ev.Kind == events.MoveZone && ev.From == state.ZStack && ev.To != state.ZStack {
		delete(e.triggerContexts, ev.Obj)
		delete(e.triggerEffectFrames, ev.Obj)
		delete(e.triggerLines, ev.Obj)
		delete(e.triggerLineSVars, ev.Obj)
		delete(e.triggerLKI, ev.Obj)
		delete(e.sacrificedLKI, ev.Obj)
		delete(e.castExiled, ev.Obj)
		delete(e.castRevealed, ev.Obj)
		delete(e.castDiscarded, ev.Obj)
		delete(e.castCounterPut, ev.Obj)
		delete(e.castTapped, ev.Obj)
		delete(e.castReturned, ev.Obj)
		delete(e.fuseTargets, ev.Obj)
		delete(e.copyTargetStage, ev.Obj)
		delete(e.copyAnswerTargets, ev.Obj)
		delete(e.castSubTargets, ev.Obj)
		delete(e.resolutionTargets, ev.Obj)
		delete(e.tpCtlChooser, ev.Obj)
		delete(e.charmTargets, ev.Obj)
		delete(e.sourceLifelinkLKI, ev.Obj)
		delete(e.sourceControllerLKI, ev.Obj)
		delete(e.sourceCharLKI, ev.Obj)
		delete(e.damageSourceLKI, ev.Obj)
	}
	if ev.Kind == events.MoveZone && lki != nil && lki.Zone == state.ZBattlefield {
		// ChangeZone's Duration$ UntilHostLeavesPlay (the Oblivion Ring /
		// Banisher Priest pattern): the exiling permanent has just left the
		// battlefield, so every card it exiled under that duration and that is
		// still in exile returns to the zone it was exiled from, under its
		// owner's control. The return is not a triggered ability, so it runs
		// BEFORE this leave event's own triggers are matched; the returned
		// cards' own ETB triggers are queued by their return move's emit. A
		// departure in a zone batch waits for the batch to close instead: the
		// return follows the whole simultaneous event (CR 610.3, 610.3d), and
		// the permanents leaving with this one are gone by then (CR 303.4g's
		// "no legal object"). events.Move prunes the marker
		// entries when their object leaves exile by any other path, so the
		// sweep can never return a card whose exile was another effect's
		// business, and a card exiled again by something else after it was
		// once returned is equally out of reach.
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone != state.ZBattlefield {
			if e.zoneBatchOpen {
				if len(o.ExileReturn) > 0 {
					e.exileReturnHeld = append(e.exileReturnHeld, ev.Obj)
				}
			} else {
				e.sweepExileReturn(ev.Obj)
			}
		}
	}
	if ev.Kind == events.MoveZone {
		// Effect-created continuous effects' move-driven lifetimes (the
		// ForgetOnMoved$/ExileOnMoved$ sweep) run after the move is applied
		// and before this event's triggers are checked, so a may-play grant's
		// remembered set is already pruned when anything downstream reads it.
		// EVERY move, not just a battlefield departure: the move the sweep
		// exists for is a may-play cast's Exile→Stack move — which rides
		// PutOnStack, the event kind a cast pushes with — and a remembered
		// card can also leave the ForgetOnMoved$ zone from the graveyard or
		// the hand.
		e.effectMoveSweep(ev)
		e.sweepEffectDelayed(ev)
	}
	if ev.Kind == events.Untap || ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
		e.expireUntilUntaps(ev.Obj)
	}
	if ev.Kind == events.ControlChange {
		e.expireAsLongAsControl(ev.Obj)
	}
	if ev.Kind == events.PutOnStack {
		e.effectMoveSweep(ev)
		e.sweepEffectDelayed(ev)
	}
	if ev.Kind == events.CounterChange {
		// Effect-created continuous effects' counter-driven lifetime (task
		// vow1; ForgetCounter$): after a counter REMOVAL is applied, a
		// remembered card whose count of the named kind reached zero leaves
		// the effect's Remembered set -- Promise of Loyalty's "for as long
		// as it has a vow counter on it". Applied before this event's own
		// triggers are checked, the same timing effectMoveSweep keeps.
		e.effectCounterSweep(ev)
		e.sweepEffectDelayed(ev)
	}
	// Damage batch (CR 510.4, Forge dealAssignedDamage): DamageDealtOnce/
	// DamageDoneOnce latch once per damage BATCH. A Damage event arriving with
	// no batch already open (combat's damageStep and effects' dealDamage calls
	// open their own; see Host.BeginDamageBatch) is a batch of one -- its own
	// batch, opened and closed around the trigger check, so the Once modes
	// fire per event rather than per turn and the queued referent's amount is
	// already the batch total. A prevented Damage never reaches here (emit
	// returned the prevention Note above), and the deferred cast-trigger arm
	// skips the trigger check entirely, so neither needs a batch.
	if e.deferCastTrigger && deferredPushKind(ev.Kind) {
		// CR 601.2i / 602.2i: the cast/activate trigger must not fire at the
		// up-front push (601.2a / 602.2a), because the spell or ability is not
		// yet cast/activated -- targets (601.2c) and payment (601.2h) still lie
		// ahead. Hold the event and its LKI so payCast's
		// fireDeferredCastTrigger / fireDeferredActivationTrigger re-walks it
		// after payment. Deferring here (rather than queueing the trigger and
		// removing it later) means the held event is never double-checked: the
		// first pass skipped it entirely, and exactly one later pass fires it.
		cp, lp := stored, lki
		e.deferredPush, e.deferredPushLKI = &cp, lp
	} else {
		if _, _, loss := lifeLoss(stored); loss && e.lifeLossBatchDepth > 0 {
			e.lifeLossBatch = append(e.lifeLossBatch, stored)
		}
		before := len(e.pendingTriggers)
		// A token mint (TokenCreate/CardToken) has no object id in the event:
		// foldTokenCreate/foldCardToken mint e.G.NextID (captured as
		// tokenMintWant before the fold) but never set ev.Obj, so the trigger
		// matcher would see Obj == 0 and ValidCard$ could not read the entering
		// token. Hand it a COPY carrying the minted id; stored itself is
		// already logged and must stay byte-identical for replay.
		check := triggerCheckEvent(stored, tokenMintWant)
		if !deferAttackTriggerCheck(stored.Kind, len(e.declaredAttackers) > 0) {
			e.checkTriggers(&check, lki, lkiPower, lkiToughness, lkiPTValid)
		}
		if stored.Kind == events.EndTurn {
			e.checkEndTurnExileTriggers(&stored)
		}
		// A pushed spell proposal's own target choice (CR 601.2c): remember
		// which queue entries it produced so abortCast can drop them if the
		// cast is reversed (CR 733.1 -- see pendingCast.proposalTriggers).
		if stored.Kind == events.TargetsChosen && e.cast != nil && e.cast.pushed &&
			!e.cast.isAbility() && stored.Obj == e.cast.stackObj && len(e.pendingTriggers) > before {
			e.cast.proposalTriggers = append(e.cast.proposalTriggers, [2]int{before, len(e.pendingTriggers)})
		}
	}
	if onlyEventBatch {
		e.closeDamageBatch()
	}
	if stored.Kind == events.PlanarRoll {
		// CR 901.4 (task planar-verbs): a completed planar-dice roll's kept
		// results have their consequences — the planeswalk face walks the
		// roller to the next plane, the chaos face makes chaos ensue on the
		// roller's current plane. The record above is logged, so the nested
		// PlanarWalk/ChaosEnsues events follow it in the log deterministically.
		e.planarRollConsequences(stored)
	}
	if ev.Kind == events.Tap && !trigmatch.TapIsEntryState(boardOf(e), ev) {
		// Recorded after the triggers above were matched, so a FirstTime$
		// trigger sees whether an EARLIER tap happened this turn.
		if e.tappedTurn == nil {
			e.tappedTurn = make(map[state.ObjID]int32)
		}
		e.tappedTurn[ev.Obj] = e.G.Turn
	}
	e.finishSourceLifelinkLKI(ev, departingSource, departingSourceLifelink, departingSourceController)
	// CR 702.179 ("Start your engines!", engine/rules/speed.go): a loss may queue
	// the active player's speed trigger (if they have speed), and a Start your
	// engines! permanent's battlefield entry starts a speed-less
	// controller's speed at 1. Checked on the FOLDED event, after
	// checkTriggers, so the queued trigger follows everything the loss itself
	// caused -- and both checks are inert for every other event. The loss
	// reaches here two ways: an explicit LifeChange with a negative amount
	// (life payment, "each player loses N life"), and a player D_DAMAGE --
	// combat damage and spell/ability damage fold straight to the life
	// total (events.Apply's Damage case) without a LifeChange, and any
	// Damage event that reaches emit has already been through prevention
	// (a prevented hit is a Note, never a Damage), so a positive player
	// Damage event here IS the life loss the rule reads.
	if (ev.Kind == events.LifeChange && ev.Amount < 0) ||
		(ev.Kind == events.Damage && ev.Obj == 0 && ev.Amount > 0 &&
			ev.Counter != "infect") {
		e.checkSpeedGain(ev)
	}
	// Ascend (CR 702.131a): the city's blessing's continuous re-check. A
	// battlefield entry (the ordinary MoveZone), a token mint (TokenCreate/
	// CardToken -- Apply mints those without a MoveZone event) or a control
	// transfer can each push a seat's permanent count over ten; the scan
	// only emits for an unblessed seat that newly qualifies, so every other
	// event reaching here is inert (engine/rules/ascend.go). The Start your
	// engines! grant (CR 702.179a) runs on the same set: each of those
	// events can hand a speed-less seat a permanent carrying the keyword,
	// and a control transfer is the case a battlefield-entry-only hook
	// cannot see (engine/rules/speed.go).
	if (ev.Kind == events.MoveZone && ev.To == state.ZBattlefield) ||
		ev.Kind == events.TokenCreate || ev.Kind == events.CardToken ||
		ev.Kind == events.ControlChange {
		e.checkSpeedStart()
		e.checkBlessingGrants()
		e.checkEnduringStoryGrants()
	}
	// E2: any genuinely state-changing event proves the game is making
	// progress, so it clears the held-out cast suppression (suppressedCast,
	// see engine.go): a declined card's option comes back the moment the
	// game does anything else, which is exactly when ending the current
	// priority window (a spell resolves, a step or turn changes) and any
	// mana/board change both land. The four kinds a declined-Delve abort
	// emits -- a Priority regrant, the KChoose decision bookkeeping, and the
	// abort Note -- are excluded, so suppression survives only as long as
	// NOTHING else is happening, which is precisely the no-progress-interval
	// it is there to bound.
	if ev.Kind != events.Priority && ev.Kind != events.DecisionAsk &&
		ev.Kind != events.DecisionMade && ev.Kind != events.Note {
		e.suppressedCast = nil
		e.castAborts = nil
		e.inertHeldOut = nil
		// CR 611.2b: a "for as long as" control effect ends the moment its
		// condition stops holding, not at the next state-based check.
		e.expireControl(controlOnEvent)
		// A GainControl$ static (Mind Control) is realized the same way:
		// ending ran above (a static grant's grantEnded reads the fresh
		// wanted set), this registers the transfers the live scan newly
		// wants. Both are no-ops unless such a static is in play.
		e.reconcileControlStatics()
	}
	if stored.Kind == events.MoveZone && stored.To == state.ZBattlefield {
		e.finishLandPlay(stored.Obj)
	}
	return stored
}

// sweepExileReturn implements ChangeZone's Duration$ UntilHostLeavesPlay
// return half: the object named by source has just left the battlefield, so
// every card it exiled under that duration and that is still in exile moves
// back to the zone it was exiled from, under its owner's control (events.Move
// gives a battlefield re-entry its owner's control, and the returned card's
// own ETB triggers queue through its return move's own emit). The marker list
// is snapshotted first: the return moves prune it underneath the loop. Entry
// order -- the order the exiles happened in -- is the return order,
// deterministic.
func (e *Engine) sweepExileReturn(source state.ObjID) {
	src := e.G.Obj(source)
	if src == nil || len(src.ExileReturn) == 0 {
		return
	}
	pending := append([]state.ExileReturnEntry(nil), src.ExileReturn...)
	for _, entry := range pending {
		o := e.G.Obj(entry.Obj)
		if o == nil || o.Zone != state.ZExile {
			continue
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: entry.Obj, From: state.ZExile, To: entry.From})
		if entry.Ret {
			e.runExileReturnAbility(source, entry.Obj)
		}
	}
}

// flushExileReturns runs the returns a closed zone batch held, as one batch of
// their own: cards returning after one event return together (CR 610.3d).
func (e *Engine) flushExileReturns() {
	held := e.exileReturnHeld
	e.exileReturnHeld = nil
	e.openZoneBatch()
	defer e.closeZoneBatch()
	for _, source := range held {
		e.sweepExileReturn(source)
	}
}

// noteTurnsTaken keeps the per-seat turn tally cache in step with the log:
// it stays valid only while every logged event passes through here, one at a
// time, and a TurnChange bumps its player's count.
func (e *Engine) noteTurnsTaken(stored *events.Event) {
	if len(e.turnsTaken) == len(e.G.Players) && e.turnsTakenEpoch == len(e.L.Events)-1 {
		if stored.Kind == events.TurnChange && int(stored.Player) < len(e.turnsTaken) {
			e.turnsTaken[stored.Player]++
		}
		e.turnsTakenEpoch++
	} else {
		e.turnsTaken = nil
		e.turnsTakenEpoch = 0
	}
}

// bookkeepingKind reports the priority bookkeeping kinds -- a Priority
// grant, a DecisionAsk and its DecisionMade answer. Together they are about
// 70% of every logged event in a search game (measured on the az bench), and
// every one of emit's per-kind hooks is a no-op for them; see
// emitBookkeeping.
func bookkeepingKind(k events.Kind) bool {
	return k == events.Priority || k == events.DecisionAsk || k == events.DecisionMade
}

// emitBookkeeping is emit for a bookkeepingKind event outside a replacement
// body (applyingReplacement rewrites a carried event, so that path stays on
// the general emit). It runs exactly the hooks of emit that are not no-ops
// for these kinds, in emit's order:
//
//   - the fold itself (foldEntryMove: no entry stage matches a non-entry
//     kind and entryCounterGrants/entryBodyCandidates/entryRiderCandidates
//     all answer "none", so it is a plain events.Emit);
//   - the turn-tally cache, the livelock watcher, the layer-3 rename and
//     layer-4 type tables, and the trigger check (with no LKI: emit takes an
//     LKI snapshot only for MoveZone/Draw/PutOnStack/ControlChange/DoorUnlock
//     and a TIME CounterChange).
//
// Every other emit hook is gated on a kind none of these is: protection and
// infect/wither (Damage), Attach and Role sweeps, the counter prohibition
// (CounterChange), entry staging (MoveZone/TokenCreate/CardToken),
// applyReplacements (Draw/LifeChange/Damage branches; replacementEvent maps
// none of these kinds, so the dispatch returns the event untouched), the
// look-back window and source-lifelink LKI (battlefield departures), the
// clone expiry (TurnFaceDown/Untap/MoveZone), the mint sinks and turn
// ledgers (AbilityPush/TokenCreate/CopyToken/StackCopy/TargetsChosen/
// ElementalBend/MoveZone), the zone-move sweeps, the damage and life-loss
// batches, the deferred cast trigger (PutOnStack), the planar roll, the tap
// ledger, speed, ascend, and the suppressedCast/control tail, which excludes
// these three kinds by name. A hook added to emit for one of these kinds must
// be added here too.
//
// It works in place: on return *ev is the stored event (Seq assigned, IDs and
// Pairs detached), with no by-value copy of the 120-byte event on the way.
// ev must point at the caller's own event, never into the log.
func (e *Engine) emitBookkeeping(ev *events.Event) {
	events.EmitPtr(e.G, e.L, ev)
	e.noteTurnsTaken(ev)
	e.loop.observeFrom(ev, e.damaging, len(e.G.Objs))
	if e.setNameInPool {
		e.refreshRenames()
	}
	if e.layer4InPool {
		e.refreshDerivedTypes()
	}
	e.checkTriggers(ev, nil, 0, 0, false)
}

// inheritCopyRecord gives the stack copy `to` its own copy of the original
// `from`'s entry in m, when there is one (CR 707.10's cost-paid objects). A
// present entry means m is non-nil.
func inheritCopyRecord[T any](m map[state.ObjID][]T, from, to state.ObjID) {
	if s, ok := m[from]; ok {
		m[to] = append([]T(nil), s...)
	}
}
