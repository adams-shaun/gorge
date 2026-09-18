package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Clone deep-copies the engine: game, log, RNG position, the pending
// decision, continuous effects, the pending-trigger queue and the trigger
// bookkeeping maps. The copy and the original then evolve independently —
// the same intents submitted to both produce the same events, chain head
// and RNG draw count (clone_test.go pins that), and nothing submitted to
// one is visible to the other. Card data (*cards.Card, *cards.SA) is
// shared: the compiled corpus is immutable once loaded.
//
// Call it only at an intent boundary — after New, Advance or Submit has
// returned, so Pending() != nil or G.Over. That is the only moment the
// fields below are not being written. A match host clones at every turn
// start to answer "view at seq N" with at most one turn of replay.
func (e *Engine) Clone() *Engine {
	c := &Engine{
		G:                   e.G.Clone(),
		L:                   e.L.Clone(),
		compiledText:        e.compiledText,
		turnsTaken:          append([]int32(nil), e.turnsTaken...),
		turnsTakenEpoch:     e.turnsTakenEpoch,
		format:              e.format,
		rng:                 e.rng.clone(),
		orderedTriggers:     e.orderedTriggers,
		applyingReplacement: e.applyingReplacement,
		choosing:            e.choosing,
		drainAwaitsTarget:   e.drainAwaitsTarget,
		drainAwaitsModes:    e.drainAwaitsModes,
		deferCastTrigger:    e.deferCastTrigger,
		// blockerRound (combat.go, Task m34): the declare-blockers round's
		// defender list and cursor, plain-value state like the mulligan round.
		// The order slice itself is never mutated (askBlockers only advances
		// the cursor), so sharing it between a clone and its original is safe,
		// the same reference-sharing Clone already practises for
		// orderedTriggers.
		blockerRound: e.blockerRound,
		// stationing (station.go): the plain-value spacecraft a pending
		// Station tap pick belongs to; zero whenever none is outstanding.
		stationing: e.stationing,
		// combatRound (combat.go, Task jj-cmb): the combat damage step's
		// pass/division continuation state. The queue, answered divisions
		// and the pending ask's option-split table are all written in place
		// as a pass progresses (handleDamageDivision, askNextDivision), so a
		// clone must own its own copies -- the cast/pendingTriggers class,
		// not the blockerRound share class.
		combatRound: cloneCombatRound(e.combatRound),
		// pregame / mulligan (rules/mulligan.go, engine.go): the London
		// round's own state. Both are documented on the Engine as fields
		// Clone copies, and neither was here — so a clone taken between the
		// opening deal and turn 1 came back with the round not running and a
		// zero mulliganRound, and re-submitting the recorded mulligan intent
		// found no seat in mulligan.seats and indexed kept[-1]. That is the
		// exact path host.viewAt takes (clone a snapshot, re-Submit the
		// intents), so every view?seq= inside the mulligan window 500'd.
		// The three slices are re-allocated, not shared: kept and taken are
		// written in place, so this is the cast/pendingTriggers class, not
		// the blockerRound class above.
		pregame:  e.pregame,
		mulligan: cloneMulligan(e.mulligan),
		opening:  cloneOpening(e.opening),
		// E2 held-out cast suppression (cast.go): the set of card ids whose
		// cast option is held out of the current window after an unpayable
		// decline. A clone taken at any intent boundary carries it forward so
		// a cloned engine offers exactly the same cast options the original
		// would (a declined card stays held out until a state change
		// re-enables it, in both engines alike). It is a plain map of object
		// ids, so it must be re-allocated, not shared.
		suppressedCast: cloneSuppressed(e.suppressedCast),
		// F05-2 per-card no-progress count (engine.go), carried alongside the
		// held-out set for the same reason: a clone taken at an intent
		// boundary must count a card's no-progress aborts exactly as the
		// live engine does, or a clone would offer (or hold out) a cast the
		// original would not. Same map-of-scalars class, so re-allocated, not
		// shared.
		castAborts:     cloneAbortCounts(e.castAborts),
		suspendedCasts: append([]state.ObjID(nil), e.suspendedCasts...),
		// The livelock watcher (rules/livelock.go): carry the Config-given
		// guard thresholds, reset the observation state. A clone only happens
		// at an intent boundary -- the only moment these fields are not being
		// written -- where the watcher holds no in-flight run or quiet count
		// worth carrying, so a fresh watcher over the same thresholds is a
		// faithful copy.
		loop: newLivelockWatcherFromGuard(e.loop.guard),
	}
	if e.riotMove != nil {
		ev := *e.riotMove
		c.riotMove = &ev
	}
	if e.pending != nil {
		d := *e.pending
		d.Options = append([]decision.Option(nil), e.pending.Options...)
		d.ResumeModes = append([]string(nil), e.pending.ResumeModes...)
		d.ResumeChoices = append([]state.Target(nil), e.pending.ResumeChoices...)
		d.ResumeChosenValid = e.pending.ResumeChosenValid
		d.ResumeRemembered = append([]state.Target(nil), e.pending.ResumeRemembered...)
		c.pending = &d
	}
	if e.resume != nil {
		// Plain value data (kind/obj plus a *cards.SA into the shared
		// immutable corpus — the same pointer class every other field here
		// shares), so one struct copy is a faithful clone (M2d-2). A clone
		// made while a mid-resolution decision is pending sees the same
		// suspended resolution the original does. The outer continuation
		// chain (fx34) is a linked list of these same value frames, so it is
		// deep-copied per-link to keep the clone independent of the
		// original's list.
		c.resume = cloneResume(e.resume)
	}
	c.controlGrants = append([]controlGrant(nil), e.controlGrants...)
	if e.continuous != nil {
		c.continuous = make([]ContinuousEffect, len(e.continuous))
		for i, ce := range e.continuous {
			ce.AddKeywords = append([]string(nil), ce.AddKeywords...)
			ce.AddTypes = append([]string(nil), ce.AddTypes...)
			if ce.RestrictParams != nil {
				m := make(map[string]string, len(ce.RestrictParams))
				for k, v := range ce.RestrictParams {
					m[k] = v
				}
				ce.RestrictParams = m
			}
			ce.Remembered = append([]state.ObjID(nil), ce.Remembered...)
			if ce.ReplacementParams != nil {
				m := make(map[string]string, len(ce.ReplacementParams))
				for k, v := range ce.ReplacementParams {
					m[k] = v
				}
				ce.ReplacementParams = m
			}
			c.continuous[i] = ce
		}
	}
	if e.pendingTriggers != nil {
		c.pendingTriggers = clonePendingTriggers(e.pendingTriggers)
	}
	if e.triggerContexts != nil {
		c.triggerContexts = make(map[state.ObjID]effects.TriggerContext, len(e.triggerContexts))
		for id, tc := range e.triggerContexts {
			c.triggerContexts[id] = tc
		}
	}
	if e.triggerLKI != nil {
		c.triggerLKI = make(map[state.ObjID]triggerObjectLKI, len(e.triggerLKI))
		for id, lki := range e.triggerLKI {
			if lki.object != nil {
				cp := lki.object.CloneDeep()
				lki.object = &cp
			}
			c.triggerLKI[id] = lki
		}
	}
	if e.sacrificedLKI != nil {
		c.sacrificedLKI = make(map[state.ObjID][]state.SacrificedInfo, len(e.sacrificedLKI))
		for id, info := range e.sacrificedLKI {
			c.sacrificedLKI[id] = append([]state.SacrificedInfo(nil), info...)
		}
	}
	if e.sourceLifelinkLKI != nil {
		c.sourceLifelinkLKI = make(map[state.ObjID]bool, len(e.sourceLifelinkLKI))
		for id, link := range e.sourceLifelinkLKI {
			c.sourceLifelinkLKI[id] = link
		}
	}
	if e.sourceControllerLKI != nil {
		c.sourceControllerLKI = make(map[state.ObjID]state.PlayerID, len(e.sourceControllerLKI))
		for id, controller := range e.sourceControllerLKI {
			c.sourceControllerLKI[id] = controller
		}
	}
	if e.damageSourceLKI != nil {
		c.damageSourceLKI = make(map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI, len(e.damageSourceLKI))
		for stack, lki := range e.damageSourceLKI {
			c.damageSourceLKI[stack] = cloneDamageSourceLKI(lki)
		}
	}
	c.triggerFireCount = cloneCounts(e.triggerFireCount)
	if e.damageBatchOpen {
		c.damageBatchOpen = true
		c.damageBatchDepth = e.damageBatchDepth
		if e.damageBatchIdx != nil {
			c.damageBatchIdx = make(map[damageBatchKey]int, len(e.damageBatchIdx))
			for k, v := range e.damageBatchIdx {
				c.damageBatchIdx[k] = v
			}
		}
		c.damageBatchLog = append([]damageBatchEntry(nil), e.damageBatchLog...)
	}
	if e.phaseUnknownNoted != nil {
		c.phaseUnknownNoted = make(map[string]bool, len(e.phaseUnknownNoted))
		for k, v := range e.phaseUnknownNoted {
			c.phaseUnknownNoted[k] = v
		}
	}
	// phaseSpecs, the unbound-face triggerEventMasks fallback and
	// triggerObjectMasks are pure syntax caches. Leave them empty: each branch
	// owns its writable caches, unlike diagnostic history.
	c.triggerObjectMasks = nil
	if e.triggerTurnFires != nil {
		c.triggerTurnFires = make(map[triggerKey]turnFires, len(e.triggerTurnFires))
		for k, v := range e.triggerTurnFires {
			c.triggerTurnFires[k] = v
		}
	}
	if e.tappedTurn != nil {
		c.tappedTurn = make(map[state.ObjID]int32, len(e.tappedTurn))
		for id, turn := range e.tappedTurn {
			c.tappedTurn[id] = turn
		}
	}
	// tapObj/tapPlayer/tapEntering and tappingForMana/tappingManaProduced are
	// emitTap's synchronous context, zero at every intent boundary.
	// triggerBefore is scoped to a batch emission/resumption, so it is nil
	// at intent boundaries and is deliberately not copied. Parked replacement
	// and commander choices and resume frames retain their own immutable
	// triggerSnapshot pointers; sharing those is safe because matching builds
	// fresh Engine scratch caches and never applies events to the snapshot.
	//
	// foreachBuf / foreachDepth (Task A2) are deliberately NOT copied: they
	// are forEachObject's scratch snapshot buffer and re-entry depth counter
	// (engine.go), live only for the duration of a single walk. A clone is
	// taken at an intent boundary (never mid-walk, so foreachDepth is zero);
	// sharing the buffer field between the original and the clone would be a
	// bug, because either one walking would clobber the other's zone snapshot
	// mid-range. Leaving both zero lets each engine grow its own buffer on
	// its next depth-0 forEachObject call.
	//
	// staticContinuous / staticEpoch are likewise deliberately NOT copied:
	// staticEffects rebuilds into the memo's reusable outer storage, so each
	// branch must own its backing array. The zero epoch forces a fresh scan
	// of the cloned board on its first active() rebuild.
	//
	// activeBuf / activeEpoch / activeVersion / activeDepth / continuousVersion
	// (engine.go, layers.go) are likewise deliberately NOT copied, with the
	// same precedent. activeBuf is active()'s shared sorted effect list and
	// activeDepth its re-entry guard; a clone must grow its own buffer, never
	// alias the original's scratch, or the copy's next rebuild would clobber
	// the original's live cache mid-range (or vice versa). activeEpoch /
	// activeVersion / continuousVersion start at zero in the fresh struct, so
	// the cloned engine misses the cache and rebuilds the identical,
	// deterministic list on its first Derived after the clone boundary.
	//
	// derivedKW / derivedTypes / derivedDepth (engine.go, layers.go) are the
	// same class: Derived's reusable keyword/type scratch buffers and their
	// re-entry guard. A clone starts with nil buffers and grows its own on its
	// first full Derived, never aliasing the original's mutable scratch — the
	// A2 buffer / C3 digest precedent, spelled out in clone.go's contract.
	if e.manaActivation != nil {
		ma := *e.manaActivation
		ma.abilities = append([]*cards.SA(nil), e.manaActivation.abilities...)
		c.manaActivation = &ma
	}
	if e.manaColorActivation != nil {
		ma := *e.manaColorActivation
		ma.triggers = clonePendingTriggers(e.manaColorActivation.triggers)
		if pt := e.manaColorActivation.trigger; pt != nil {
			ma.trigger = &clonePendingTriggers([]pendingTrigger{*pt})[0]
		}
		c.manaColorActivation = &ma
	}
	if e.manaDiscardActivation != nil {
		ma := *e.manaDiscardActivation
		ma.cost.Sac = append([]CostPart(nil), e.manaDiscardActivation.cost.Sac...)
		ma.cost.Discard = append([]CostPart(nil), e.manaDiscardActivation.cost.Discard...)
		ma.cost.SubCounter = append([]CostPart(nil), e.manaDiscardActivation.cost.SubCounter...)
		ma.cost.Exile = append([]CostPart(nil), e.manaDiscardActivation.cost.Exile...)
		ma.cost.Reveal = append([]CostPart(nil), e.manaDiscardActivation.cost.Reveal...)
		ma.cost.Behold = append([]CostPart(nil), e.manaDiscardActivation.cost.Behold...)
		ma.cost.TapPermanent = append([]CostPart(nil), e.manaDiscardActivation.cost.TapPermanent...)
		ma.cost.Blight = append([]CostPart(nil), e.manaDiscardActivation.cost.Blight...)
		ma.sacs = append([]state.ObjID(nil), e.manaDiscardActivation.sacs...)
		ma.discards = append([]state.ObjID(nil), e.manaDiscardActivation.discards...)
		ma.exiles = append([]state.ObjID(nil), e.manaDiscardActivation.exiles...)
		c.manaDiscardActivation = &ma
	}
	if e.manaUnlessActivation != nil {
		ma := *e.manaUnlessActivation
		ma.triggers = clonePendingTriggers(e.manaUnlessActivation.triggers)
		ma.payers = append([]state.PlayerID(nil), e.manaUnlessActivation.payers...)
		c.manaUnlessActivation = &ma
	}
	if e.unlessPayment != nil {
		u := *e.unlessPayment
		u.cost.Sac = append([]CostPart(nil), e.unlessPayment.cost.Sac...)
		u.cost.Discard = append([]CostPart(nil), e.unlessPayment.cost.Discard...)
		u.cost.SubCounter = append([]CostPart(nil), e.unlessPayment.cost.SubCounter...)
		u.cost.Draw = append([]CostPart(nil), e.unlessPayment.cost.Draw...)
		u.sacs = append([]state.ObjID(nil), e.unlessPayment.sacs...)
		u.discards = append([]state.ObjID(nil), e.unlessPayment.discards...)
		u.ctx = cloneUnlessCtx(e.unlessPayment.ctx)
		u.rp = cloneResume(e.unlessPayment.rp)
		c.unlessPayment = &u
	}
	if e.cumulative != nil {
		cu := *e.cumulative
		cu.amount.Sac = append([]CostPart(nil), e.cumulative.amount.Sac...)
		cu.amount.Discard = append([]CostPart(nil), e.cumulative.amount.Discard...)
		cu.amount.SubCounter = append([]CostPart(nil), e.cumulative.amount.SubCounter...)
		cu.amount.AddCounter = append([]CostPart(nil), e.cumulative.amount.AddCounter...)
		cu.amount.Exile = append([]CostPart(nil), e.cumulative.amount.Exile...)
		cu.amount.Reveal = append([]CostPart(nil), e.cumulative.amount.Reveal...)
		cu.amount.Behold = append([]CostPart(nil), e.cumulative.amount.Behold...)
		cu.amount.TapPermanent = append([]CostPart(nil), e.cumulative.amount.TapPermanent...)
		cu.amount.Blight = append([]CostPart(nil), e.cumulative.amount.Blight...)
		cu.amount.Hybrid = append([]ManaPair(nil), e.cumulative.amount.Hybrid...)
		cu.amount.Twobrid = append([]Twobrid(nil), e.cumulative.amount.Twobrid...)
		cu.amount.HybridPhyrexian = append([]HybridPhyrexian(nil), e.cumulative.amount.HybridPhyrexian...)
		cu.amount.Phyrexian = append([]byte(nil), e.cumulative.amount.Phyrexian...)
		cu.amount.Unknown = append([]string(nil), e.cumulative.amount.Unknown...)
		if e.cumulative.action != nil {
			action := *e.cumulative.action
			cu.action = &action
		}
		c.cumulative = &cu
	}
	if e.triggerCost != nil {
		tc := *e.triggerCost
		tc.resume = cloneResume(e.triggerCost.resume)
		tc.amount.Sac = append([]CostPart(nil), e.triggerCost.amount.Sac...)
		tc.amount.Discard = append([]CostPart(nil), e.triggerCost.amount.Discard...)
		tc.amount.SubCounter = append([]CostPart(nil), e.triggerCost.amount.SubCounter...)
		tc.amount.AddCounter = append([]CostPart(nil), e.triggerCost.amount.AddCounter...)
		tc.amount.Exile = append([]CostPart(nil), e.triggerCost.amount.Exile...)
		tc.amount.Reveal = append([]CostPart(nil), e.triggerCost.amount.Reveal...)
		tc.amount.Behold = append([]CostPart(nil), e.triggerCost.amount.Behold...)
		tc.amount.TapPermanent = append([]CostPart(nil), e.triggerCost.amount.TapPermanent...)
		tc.amount.Blight = append([]CostPart(nil), e.triggerCost.amount.Blight...)
		tc.amount.Hybrid = append([]ManaPair(nil), e.triggerCost.amount.Hybrid...)
		tc.amount.Twobrid = append([]Twobrid(nil), e.triggerCost.amount.Twobrid...)
		tc.amount.HybridPhyrexian = append([]HybridPhyrexian(nil), e.triggerCost.amount.HybridPhyrexian...)
		tc.amount.Phyrexian = append([]byte(nil), e.triggerCost.amount.Phyrexian...)
		tc.amount.Unknown = append([]string(nil), e.triggerCost.amount.Unknown...)
		c.triggerCost = &tc
	}
	if e.wardMana != nil {
		wm := *e.wardMana
		c.wardMana = &wm
	}
	if e.cast != nil {
		pc := *e.cast
		pc.cost.Sac = append([]CostPart(nil), e.cast.cost.Sac...)
		pc.cost.Discard = append([]CostPart(nil), e.cast.cost.Discard...)
		pc.cost.SubCounter = append([]CostPart(nil), e.cast.cost.SubCounter...)
		pc.cost.Exile = append([]CostPart(nil), e.cast.cost.Exile...)
		pc.cost.Reveal = append([]CostPart(nil), e.cast.cost.Reveal...)
		pc.cost.Behold = append([]CostPart(nil), e.cast.cost.Behold...)
		pc.cost.TapPermanent = append([]CostPart(nil), e.cast.cost.TapPermanent...)
		pc.cost.Blight = append([]CostPart(nil), e.cast.cost.Blight...)
		pc.cost.Draw = append([]CostPart(nil), e.cast.cost.Draw...)
		pc.cost.LifeX = append([]CostPart(nil), e.cast.cost.LifeX...)
		pc.cost.DamageYou = append([]CostPart(nil), e.cast.cost.DamageYou...)
		pc.cost.Energy = append([]CostPart(nil), e.cast.cost.Energy...)
		pc.cost.Return = append([]CostPart(nil), e.cast.cost.Return...)
		pc.cost.Hybrid = append([]ManaPair(nil), e.cast.cost.Hybrid...)
		pc.cost.Phyrexian = append([]byte(nil), e.cast.cost.Phyrexian...)
		pc.cost.Twobrid = append([]Twobrid(nil), e.cast.cost.Twobrid...)
		pc.cost.HybridPhyrexian = append([]HybridPhyrexian(nil), e.cast.cost.HybridPhyrexian...)
		pc.mods.reduces = append([]costMod(nil), e.cast.mods.reduces...)
		pc.mods.raises = append([]int32(nil), e.cast.mods.raises...)
		pc.delve = append([]state.ObjID(nil), e.cast.delve...)
		pc.sacs = append([]state.ObjID(nil), e.cast.sacs...)
		pc.discards = append([]state.ObjID(nil), e.cast.discards...)
		pc.exiles = append([]state.ObjID(nil), e.cast.exiles...)
		pc.returns = append([]state.ObjID(nil), e.cast.returns...)
		pc.reveals = append([]state.ObjID(nil), e.cast.reveals...)
		pc.beholds = append([]state.ObjID(nil), e.cast.beholds...)
		pc.taps = append([]state.ObjID(nil), e.cast.taps...)
		pc.blights = append([]state.ObjID(nil), e.cast.blights...)
		pc.preModes = append([]string(nil), e.cast.preModes...)
		pc.preSuppress = cloneSuppressed(e.cast.preSuppress)
		pc.preAborts = cloneAbortCounts(e.cast.preAborts)
		c.cast = &pc
		// The held-back cast trigger (CR 601.2i, cast.go): a clone taken at an
		// intent boundary while a cast is suspended (its target/choose decision
		// pending) must carry the deferred PutOnStack event and its LKI so that
		// re-Submitting the target answer still fires the cast trigger in the
		// clone, exactly as it does in the original. The event is a value; the
		// LKI is a read-only snapshot safely shared like every other immutable
		// Object pointer in this function.
		if e.deferredPush != nil {
			ev := *e.deferredPush
			c.deferredPush = &ev
		}
		c.deferredPushLKI = e.deferredPushLKI
	}
	if e.cmdZone != nil {
		// The parked commander zone changes (CR 903.9, Task m32): a clone
		// taken while a KCommanderZone decision is outstanding must carry the
		// same queue the original does, or answering the copied decision
		// would find no parked move and the commander would never move.
		// Plain value entries, so one slice copy is a faithful clone.
		c.cmdZone = append([]cmdZoneMove(nil), e.cmdZone...)
	}
	if e.replChoices != nil {
		// Parked replacement choices (MoveZone/ProduceMana/BeginPhase order,
		// replacement mana colour and optional phase apply/decline): same class
		// as cmdZone. Event/scalar data copies by value;
		// candidate pointers share immutable corpus data, while every mutable
		// bookkeeping slice is re-allocated for the clone.
		c.replChoices = make([]replChoice, len(e.replChoices))
		for i, rc := range e.replChoices {
			rc.cands = append([]replMatch(nil), rc.cands...)
			rc.used = append([]replMatch(nil), rc.used...)
			rc.applied = append([]bool(nil), rc.applied...)
			rc.appliedRepls = append([]replMatch(nil), rc.appliedRepls...)
			rc.applicable = append([]int(nil), rc.applicable...)
			if rc.untap != nil {
				resume := *rc.untap
				rc.untap = &resume
			}
			c.replChoices[i] = rc
		}
	}
	c.madnessChoices = append([]events.Event(nil), e.madnessChoices...)
	return c
}

// cloneCounts copies a trigger-bookkeeping map, preserving nil (trigger.go
// lazily allocates these on first use and checks for nil itself).
func cloneCounts(m map[triggerKey]int32) map[triggerKey]int32 {
	if m == nil {
		return nil
	}
	out := make(map[triggerKey]int32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneSuppressed copies the held-out cast set (suppressedCast, engine.go),
// preserving nil (cast.go allocates it lazily; a nil set reads as empty).
func cloneSuppressed(m map[state.ObjID]bool) map[state.ObjID]bool {
	if m == nil {
		return nil
	}
	out := make(map[state.ObjID]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneAbortCounts copies the F05-2 per-card no-progress count
// (castAborts, engine.go), preserving nil; the lazily-allocated map is
// created by abortCast and a nil map reads as an empty count.
func cloneAbortCounts(m map[state.ObjID]int32) map[state.ObjID]int32 {
	if m == nil {
		return nil
	}
	out := make(map[state.ObjID]int32, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// cloneMulligan deep-copies the London round: the phase flags and counters
// are plain values, but seats/kept/taken are written in place while the
// round runs (rules/mulligan.go), so the copy must own its own arrays.
func cloneMulligan(m mulliganRound) mulliganRound {
	m.seats = append([]state.PlayerID(nil), m.seats...)
	m.kept = append([]bool(nil), m.kept...)
	m.taken = append([]int(nil), m.taken...)
	return m
}

// clonePendingTriggers gives a clone ownership of the mutable context carried
// by both the ordinary trigger queue and a CR 605.3b batch parked on a mana
// colour choice. Card and SA pointers remain shared immutable corpus data.
func clonePendingTriggers(src []pendingTrigger) []pendingTrigger {
	if src == nil {
		return nil
	}
	out := make([]pendingTrigger, len(src))
	for i, pt := range src {
		pt.Ctx.Targets = append([]state.Target(nil), pt.Ctx.Targets...)
		pt.Ctx.Remembered = append([]state.Target(nil), pt.Ctx.Remembered...)
		if pt.Ctx.SVars != nil {
			m := make(map[string]string, len(pt.Ctx.SVars))
			for k, v := range pt.Ctx.SVars {
				m[k] = v
			}
			pt.Ctx.SVars = m
		}
		if pt.Ctx.LKI != nil {
			lki := pt.Ctx.LKI.CloneDeep()
			pt.Ctx.LKI = &lki
		}
		out[i] = pt
	}
	return out
}

// cloneCombatRound deep-copies the combat damage step's continuation state
// (combat.go, Task jj-cmb): the division queue, answered divisions and the
// pending ask's option-split table are all written in place while a pass
// progresses, so a clone must own its own arrays rather than alias the
// original's.
func cloneCombatRound(cr combatRound) combatRound {
	cr.queue = append([]state.ObjID(nil), cr.queue...)
	cr.assignments = append([]assignment(nil), cr.assignments...)
	if cr.done != nil {
		done := make([]divChoice, len(cr.done))
		for i, dc := range cr.done {
			done[i] = divChoice{attacker: dc.attacker, amounts: append([]int32(nil), dc.amounts...)}
		}
		cr.done = done
	}
	if cr.askOptions != nil {
		table := make([][]int32, len(cr.askOptions))
		for i, row := range cr.askOptions {
			table[i] = append([]int32(nil), row...)
		}
		cr.askOptions = table
	}
	return cr
}

// cloneResume deep-copies a suspended resolution's resume chain (fx34): each
// link is plain value data (kind/obj plus a *cards.SA into the shared,
// immutable corpus), but the outer continuation chain is a linked list this
// cloned engine must own so it can resume outward independently of the
// original's traversal.
func cloneResume(rp *resumePoint) *resumePoint {
	if rp == nil {
		return nil
	}
	cp := *rp
	cp.choices = append([]state.Target(nil), rp.choices...)
	cp.chosenValid = rp.chosenValid
	cp.remembered = append([]state.Target(nil), rp.remembered...)
	cp.loopRemembered = append([]state.Target(nil), rp.loopRemembered...)
	if rp.repeat != nil {
		cur := *rp.repeat
		cur.subjects = append([]state.Target(nil), rp.repeat.subjects...)
		cur.last = append([]state.Target(nil), rp.repeat.last...)
		cp.repeat = &cur
	}
	cp.outer = cloneResume(rp.outer)
	return &cp
}
