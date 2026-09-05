// replacement.go applies R: line replacement effects ahead of an event's
// own logging, from engine.go's emit. applyReplacements finds the single
// best-fitting match via forEachObject's deterministic scan and
// replacementMatches's predicate, then either runs ReplaceWith$ in place of
// the original event or -- for ReplacementResult$ Updated -- runs the
// original event first and ReplaceWith$ after.
package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// applyReplacements is called from emit before the event is logged. The
// single M1 replacement event is "Moved" (R:Event$ Moved), which applies to
// a MoveZone event and honours Origin$, Destination$, ValidCard$ and
// ReplaceWith$.
//
// At most one matching replacement applies, in forEachObject's own
// deterministic scan order: CR 616's full multi-replacement, player-chooses-
// order algorithm is not modeled, which is an M1 simplification, not an
// oversight. A match resolves ReplaceWith$'s ability (which itself emits
// whatever events its own primitives call for -- Ruling: applyingReplacement
// is already true by then, so those nested emits skip this check entirely
// rather than potentially replacing themselves forever).
//
// Task 29 (Ruling T26-a): what happens to the ORIGINAL event once a
// replacement matches depends on Forge's ReplacementResult$, which this used
// to ignore entirely -- every match was treated as ReplacementResult$
// Replaced, discarding the original event unconditionally. That is right for
// "Replaced" (and for no ReplacementResult$ at all: Task 22's four pins were
// measured against fixtures with neither, and must keep reading as
// Replaced), but wrong for ReplacementResult$ Updated -- Forge's idiom for
// "the event still happens, augmented" (CR 616.1's "an effect that modifies
// how an event occurs"), which is by far the dominant shape in the corpus:
// 838 of 842 ReplacementResult$-bearing R: lines say Updated, and 835 of
// those are exactly this "enters the battlefield tapped" pattern (Hallowed
// Fountain, Celestial Colonnade, Geralf's Messenger, ...). Treating Updated
// as a full replace discarded the permanent's own MoveZone onto the
// battlefield, so it never left the stack and resolveTop kept re-resolving
// the same object forever (see Task 26's report and the resolveTop guard
// below for the other half of this fix).
func (e *Engine) applyReplacements(ev events.Event) (events.Event, bool) {
	if ev.Kind != events.MoveZone {
		return ev, false
	}
	var matchID state.ObjID
	var matchRepl *cards.Repl
	e.forEachObject(func(id state.ObjID) {
		if matchRepl != nil {
			return
		}
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		for i := range f.Repls {
			if e.replacementMatches(f.Repls[i], id, ev) {
				matchID, matchRepl = id, &f.Repls[i]
				return
			}
		}
	})
	if matchRepl == nil || matchRepl.With == nil {
		return ev, false
	}

	o := e.G.Obj(matchID)
	ctx := &effects.Ctx{Source: matchID, Controller: o.Controller,
		Remembered: []state.Target{{Obj: ev.Obj}}}
	if f := o.Face(); f != nil {
		effects.SetSVars(ctx, f.SVars)
	}

	// Review finding M-6 (Task 29 fix round 1): matchRepl.Params reads a
	// map built by cards/parse.go's parseParams, which trims both key and
	// value -- so an exact, case-sensitive "Updated" compare is deliberate,
	// not an oversight that happens to work. Forge's own corpus is uniform
	// here (every ReplacementResult$ occurrence across the shipped cards
	// spells it exactly this way), and a laxer compare (case-fold, trim
	// again, ==prefix) would silently paper over a future corpus value this
	// build has never seen rather than surfacing it -- reading today's
	// exact spelling is what makes a drift visible instead of quietly
	// falling back to "Replaced" behaviour for a card that meant "Updated".
	if matchRepl.Params["ReplacementResult"] == "Updated" {
		// Apply the ORIGINAL event first, through the same events.Emit +
		// checkTriggers pair emit itself uses for an unreplaced event --
		// but calling them directly here, rather than routing back through
		// e.emit, is what keeps this from re-running replacement matching
		// on the event it just matched: CR 616.1, a replacement effect
		// applies only once to a given event. checkTriggers still runs
		// unconditionally (it never checks applyingReplacement), so an ETB
		// trigger watching this same Move FIRES exactly as it would for an
		// unreplaced entry.
		//
		// Review finding I-3: firing is not the whole story. checkTriggers
		// evaluates each Trigger's ValidCard$ predicate against the object's
		// state as of RIGHT NOW -- before ReplaceWith$ below has run -- so a
		// trigger that inspects the very characteristic this replacement is
		// about to change (a ValidCard$ Card.tapped/Card.untapped predicate
		// watching an "enters tapped" permanent, say) matches against the
		// UNTAPPED state, i.e. the opposite of the state the permanent is
		// left in an instant later. Forge itself models Ctx as "the event,
		// already modified by ReplaceWith$" and matches triggers against
		// that; applying the original event verbatim and patching it
		// afterward, the way this build's applyReplacements works, cannot
		// reproduce that ordering without restructuring how the whole
		// replacement/trigger pipeline threads state, which is out of this
		// task's scope. Measured (8 corpus cards carry a tapped/untapped
		// ChangesZone predicate; none in a repo deck, so unreachable from
		// the acceptance suite): this is a real, if narrow, M1 approximation
		// of CR 616.1, not a hypothetical.
		stored := events.Emit(e.G, e.L, ev)
		e.checkTriggers(stored, nil)

		// THEN resolve ReplaceWith$. The order is measured, not stylistic:
		// effects.Resolve's Tap primitive (effects/combatfx.go effTap)
		// only taps an object already on the battlefield (it treats one
		// anywhere else, including still on the stack, as nothing to do)
		// and events.Apply's Move only assigns battlefield-entry fields
		// (SummonSick, Timestamp, ...) for the destination named in the
		// Move it is actually given, so a Tap attempted before the object
		// has really moved is silently a no-op, not merely undone by a
		// later reset. Running the Move first means DBTap (or whatever
		// ReplaceWith$ does) lands on an object already in its new zone,
		// so "enters tapped" actually sticks.
		e.applyingReplacement = true
		effects.Resolve(e, ctx, matchRepl.With)
		e.applyingReplacement = false
		return stored, true
	}

	// Anything else -- ReplacementResult$ absent or "Replaced", or any
	// other value -- keeps today's behaviour: the original event is
	// discarded and only ReplaceWith$'s own effect happens. Task 22's four
	// pins are exactly this shape (no ReplacementResult$ at all) and must
	// not move.
	e.applyingReplacement = true
	effects.Resolve(e, ctx, matchRepl.With)
	e.applyingReplacement = false
	return ev, true
}

// replacementMatches implements R:Event$ Moved's own Origin$/Destination$/
// ValidCard$ parameters -- the same shape as zoneChangeMatches, for a
// replacement instead of a trigger.
func (e *Engine) replacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	if r.Event != "Moved" || ev.Kind != events.MoveZone {
		return false
	}
	if o, ok := r.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
		return false
	}
	if d, ok := r.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	if v, ok := r.Params["ValidCard"]; ok {
		return effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source)
	}
	return true
}
