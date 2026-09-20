package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// whenspentUnit is one when-spent mana batch (TriggersWhenSpent$ on the
// producing AB$ Mana — task mordorparams1, Path of Ancestry's "When that
// mana is spent to cast a creature spell that shares a creature type with
// your commander, scry 1") that a payment just consumed: trig is the SVar
// name of the mana ability's trigger definition, source the permanent whose
// ability produced the mana. The capture lives in Engine.whenspentSpend and
// is drained once per spell-cost payment (payManaCastSpent ->
// fireWhenspentSpent); every payment resets the list at
// emitRestrictedManaSpend's entry, so a stale unit never reaches a later
// payment's read.
type whenspentUnit struct {
	trig   string
	source state.ObjID
}

// fireWhenspentSpent drains whenspentSpend — the when-spent batches the
// just-completed SPELL-cost payment consumed — and queues each source's
// TriggersWhenSpent$ trigger definition against the paying cast, through the
// granted-trigger pipeline (the checkGrantedStaticTriggersUsing shape: the
// queue walk evaluates the definition exactly like a face trigger, then
// pushTrigger mints the stack object through events.GrantTriggerPush, whose
// Execute$ name events.Apply resolves from the SOURCE object's own SVar
// table — the live==replay contract).
//
// The event context is the cast's own PutOnStack (CR 601.2a), which is in
// the log before payment (601.2h) — the exact event a printed "whenever you
// cast" trigger matched — so the ordinary matcher (triggerMatches ->
// spellCastMatches) and the ordinary referent capture (triggerReferents:
// TriggerCard = the cast spell) run unmodified.
//
// Determinism/replay: every input is engine state or the log (the consumed
// batches' records, the source face, the cast's push event), so a replay
// re-derives the identical queue; no new event kind or field is introduced —
// the trigger rides GrantTriggerPush, the granted-trigger precedent.
//
// Attribution approximation (disclosed): the pool's provenance is consumed
// FIFO by insertion order, so when identical-colour when-spent units and
// plain units coexist, a spend may consume a different unit's provenance
// than the player "mentally" spent; several units of ONE producer's
// production consumed by one cast queue ONE trigger (dedup below), while
// units of two DIFFERENT producers queue two.
func (e *Engine) fireWhenspentSpent(pc *pendingCast) {
	if len(e.whenspentSpend) == 0 {
		return
	}
	units := e.whenspentSpend
	e.whenspentSpend = nil
	// The paying cast's push event. No push (a synthetic payment, a
	// cast-shaped flow the engine drives without one) queues nothing.
	ev, ok := e.latestPutOnStackOf(pc.card)
	if !ok {
		return
	}
	type seenKey struct {
		source state.ObjID
		trig   string
	}
	dedup := make(map[seenKey]bool, len(units))
	for _, u := range units {
		dk := seenKey{source: u.source, trig: u.trig}
		if dedup[dk] {
			continue
		}
		dedup[dk] = true
		src := e.G.Obj(u.source)
		if src == nil || src.Face() == nil {
			continue
		}
		raw := src.Face().SVars[u.trig]
		if raw == "" {
			continue
		}
		t, ok := cards.ParseTriggerLine(raw)
		if !ok || t.Mode != "SpellCast" {
			continue
		}
		// The live==replay gate (grantedTriggerExecute's contract):
		// events.Apply's GrantTriggerPush resolves the Execute$ body from
		// the SOURCE object's own SVar table; a body it cannot produce
		// never queues — minting an ability a replay cannot rebuild would
		// break the replayable-log invariant.
		exec := strings.TrimSpace(t.Params["Execute"])
		if exec == "" {
			continue
		}
		body := grantedTriggerExecute(src, exec)
		if body == nil {
			continue
		}
		if !e.triggerMatches(t, u.source, ev, nil) {
			continue
		}
		key := triggerKey{Source: u.source, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue // cascade bound: see maxTriggerFires.
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     u.source,
			Controller: src.Controller,
			Idx:        -1,
			SA:         body,
			Granted:    true,
			Execute:    exec,
			Ctx: effects.Ctx{
				Source:         u.source,
				Controller:     src.Controller,
				TriggerContext: e.triggerReferents(t, u.source, ev, nil),
			},
		})
	}
}

// latestPutOnStackOf scans the log backward for the most recent PutOnStack
// event naming obj — the cast's CR 601.2a push, the event a printed
// "whenever you cast" trigger matched. False when obj never reached the
// stack.
func (e *Engine) latestPutOnStackOf(obj state.ObjID) (events.Event, bool) {
	if obj == 0 {
		return events.Event{}, false
	}
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == obj {
			return ev, true
		}
	}
	return events.Event{}, false
}
