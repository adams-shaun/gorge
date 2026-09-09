package rules

import (
	"github.com/adams-shaun/gorge/decision"
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
		format:              e.format,
		rng:                 e.rng.clone(),
		orderedTriggers:     e.orderedTriggers,
		applyingReplacement: e.applyingReplacement,
		choosing:            e.choosing,
		drainAwaitsTarget:   e.drainAwaitsTarget,
		// blockerRound (combat.go, Task m34): the declare-blockers round's
		// defender list and cursor, plain-value state like the mulligan round.
		// The order slice itself is never mutated (askBlockers only advances
		// the cursor), so sharing it between a clone and its original is safe,
		// the same reference-sharing Clone already practises for
		// orderedTriggers.
		blockerRound: e.blockerRound,
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
		// E2 held-out cast suppression (cast.go): the set of card ids whose
		// cast option is held out of the current window after an unpayable
		// decline. A clone taken at any intent boundary carries it forward so
		// a cloned engine offers exactly the same cast options the original
		// would (a declined card stays held out until a state change
		// re-enables it, in both engines alike). It is a plain map of object
		// ids, so it must be re-allocated, not shared.
		suppressedCast: cloneSuppressed(e.suppressedCast),
	}
	if e.pending != nil {
		d := *e.pending
		d.Options = append([]decision.Option(nil), e.pending.Options...)
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
			c.continuous[i] = ce
		}
	}
	if e.pendingTriggers != nil {
		c.pendingTriggers = make([]pendingTrigger, len(e.pendingTriggers))
		for i, pt := range e.pendingTriggers {
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
			c.pendingTriggers[i] = pt
		}
	}
	c.triggerFireCount = cloneCounts(e.triggerFireCount)
	c.damageOnceFired = cloneCounts(e.damageOnceFired)
	// foreachBuf / foreachDepth (Task A2) are deliberately NOT copied: they
	// are forEachObject's scratch snapshot buffer and re-entry depth counter
	// (engine.go), live only for the duration of a single walk. A clone is
	// taken at an intent boundary (never mid-walk, so foreachDepth is zero);
	// sharing the buffer field between the original and the clone would be a
	// bug, because either one walking would clobber the other's zone snapshot
	// mid-range. Leaving both zero lets each engine grow its own buffer on
	// its next depth-0 forEachObject call.
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
	if e.cast != nil {
		pc := *e.cast
		pc.cost.Sac = append([]CostPart(nil), e.cast.cost.Sac...)
		pc.cost.SubCounter = append([]CostPart(nil), e.cast.cost.SubCounter...)
		pc.delve = append([]state.ObjID(nil), e.cast.delve...)
		pc.sacs = append([]state.ObjID(nil), e.cast.sacs...)
		c.cast = &pc
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
		// The parked CR 616.1 replacement-order choices: same class as
		// cmdZone. Plain value entries (an events.Event plus a []replMatch
		// whose *cards.Repl pointers are shared corpus data), so one slice
		// copy is a faithful clone; the candidate slice is re-allocated so
		// the clone owns its own.
		c.replChoices = make([]replChoice, len(e.replChoices))
		for i, rc := range e.replChoices {
			rc.cands = append([]replMatch(nil), rc.cands...)
			c.replChoices[i] = rc
		}
	}
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

// cloneMulligan deep-copies the London round: the phase flags and counters
// are plain values, but seats/kept/taken are written in place while the
// round runs (rules/mulligan.go), so the copy must own its own arrays.
func cloneMulligan(m mulliganRound) mulliganRound {
	m.seats = append([]state.PlayerID(nil), m.seats...)
	m.kept = append([]bool(nil), m.kept...)
	m.taken = append([]int(nil), m.taken...)
	return m
}

// cloneCombatRound deep-copies the combat damage step's continuation state
// (combat.go, Task jj-cmb): the division queue, answered divisions and the
// pending ask's option-split table are all written in place while a pass
// progresses, so a clone must own its own arrays rather than alias the
// original's.
func cloneCombatRound(cr combatRound) combatRound {
	cr.queue = append([]state.ObjID(nil), cr.queue...)
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
	cp.outer = cloneResume(rp.outer)
	return &cp
}
