package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// lifelinkGain is one pending lifelink accumulation for the open damage
// batch: the total life one source makes one player gain from the damage
// dealt simultaneously in that batch (CR 702.15e with CR 119.9). combat
// records whether the amount came from a combat-damage pass, so the merged
// event is emitted under the same synchronous damage context the per-hit
// emit used to run under.
type lifelinkGain struct {
	source state.ObjID
	player state.PlayerID
	amount int32
	combat bool
}

// GainFromLifelink is the effects-side hook for CR 702.15a/702.15e: damage a
// lifelink source deals inside an open damage batch accumulates per (source,
// player) and is emitted as ONE LifeChange when the batch closes, so
// "whenever you gain life" sees one event of the total (CR 119.9). Outside a
// batch the gain is emitted at once, as before.
func (e *Engine) GainFromLifelink(source state.ObjID, player state.PlayerID, amount int32) {
	e.gainFromLifelink(source, player, amount, false)
}

// gainFromLifelink accumulates one lifelink amount. CR 119.9: gaining 0 is no
// life-gain event at all, so a non-positive amount is dropped (a hit replaced
// down to 0 gains nothing). With no open batch it emits immediately through
// emitLifelinkGain; otherwise it adds to the entry with the same source AND
// player, or appends a new one (CR 702.15e: one source's simultaneous damage
// is one gain, different sources stay separate gains). A batch whose damage
// replacement is still parked has closed its other bookkeeping but keeps
// lifelinkBatchOpen set, so the parked hit's gain merges into the same event.
func (e *Engine) gainFromLifelink(source state.ObjID, player state.PlayerID, amount int32, combat bool) {
	if amount <= 0 {
		return
	}
	if e.damageBatchDepth == 0 && !e.lifelinkBatchOpen {
		e.emitLifelinkGain(lifelinkGain{source: source, player: player, amount: amount, combat: combat})
		return
	}
	for i := range e.lifelinkBatch {
		if e.lifelinkBatch[i].source == source && e.lifelinkBatch[i].player == player {
			e.lifelinkBatch[i].amount += amount
			return
		}
	}
	e.lifelinkBatch = append(e.lifelinkBatch,
		lifelinkGain{source: source, player: player, amount: amount, combat: combat})
}

// emitLifelinkGain emits one merged LifeChange under the synchronous damage
// context of its source, so livelock observation and a parked life
// replacement choice see the same source the old per-hit emit ran under
// (engine/rules/emit.go, engine/rules/replacement_life.go).
func (e *Engine) emitLifelinkGain(g lifelinkGain) {
	damaging, combatDamaging, override := e.damaging, e.combatDamaging, e.dmgSrcOverride
	if g.combat {
		e.damaging, e.combatDamaging = g.source, true
	} else {
		e.dmgSrcOverride = g.source
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: g.player, Amount: g.amount})
	e.damaging, e.combatDamaging, e.dmgSrcOverride = damaging, combatDamaging, override
}

// flushLifelinkBatch emits the pending gains in first-landed order, one
// LifeChange per (source, player) entry (CR 702.15e). Each entry is removed
// before it is emitted, so a gain that parks a decision leaves the rest of
// the queue intact. With stopOnAsk, a parked CR 616.1 replacement-order ask
// stops the flush and keeps the remaining entries for the resume; the caller
// reports false. A fully flushed batch is cleared to nil.
func (e *Engine) flushLifelinkBatch(stopOnAsk bool) bool {
	for len(e.lifelinkBatch) > 0 {
		g := e.lifelinkBatch[0]
		e.lifelinkBatch = append(e.lifelinkBatch[:0], e.lifelinkBatch[1:]...)
		e.emitLifelinkGain(g)
		if stopOnAsk && e.pending != nil && e.pending.Kind == decision.KReplacement {
			return false
		}
	}
	e.lifelinkBatch = nil
	return true
}

// parkedDamageReplacement reports whether a CR 616.1 damage order choice is
// still queued. The front entry is the posed ask while it is outstanding, so
// scanning the queue covers both the pending decision and the ones behind it.
func (e *Engine) parkedDamageReplacement() bool {
	for i := range e.replChoices {
		if e.replChoices[i].kind == replChoiceDamage {
			return true
		}
	}
	return false
}

// flushDeferredLifelink flushes the accumulation closeDamageBatch kept open
// across a parked damage replacement. It runs once the parked damage queue has
// drained, so every hit of the batch has paid its lifelink and the merged
// per-(source, player) events are complete (CR 702.15e).
func (e *Engine) flushDeferredLifelink() {
	if !e.lifelinkBatchOpen {
		return
	}
	e.lifelinkBatchOpen = false
	e.flushLifelinkBatch(false)
}
