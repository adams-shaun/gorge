// replacement.go applies R: line replacement effects ahead of an event's
// own logging, from engine.go's emit. applyReplacements finds the single
// best-fitting match via forEachObject's deterministic scan and
// replacementMatches's predicate, then either runs ReplaceWith$ in place of
// the original event or -- for ReplacementResult$ Updated -- runs the
// original event first and ReplaceWith$ after.
package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
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
	// CR 903.9 (Task m32): a commander about to be put into its owner's
	// graveyard, hand or library from anywhere, or exiled from anywhere, may
	// instead be put into the command zone by its OWNER. This is a
	// replacement effect exactly like the R: lines below -- it applies before
	// the object would change zones, so the commander never touches the
	// destination -- but it is a construct rule, not a card line, so it is
	// matched first (before any card-text replacement the same move might
	// also match, which per CR 616.1 would then apply to whichever zone
	// change actually happens). Matching parks the event: the owner is asked
	// (decision.KCommanderZone) and the parked move is emitted for real only
	// when the answer arrives -- to the command zone on an accept, verbatim
	// on a decline -- so the log always carries the zone change that actually
	// happened and a log-only replay reproduces it. The choice itself is a
	// player decision recorded as an Intent plus the DecisionAsk/DecisionMade
	// events every ask produces. Returning handled discards the original
	// event, which is exactly right: nothing has happened yet, and whatever
	// happens is the owner's answer, not this park.
	if e.commanderZoneReplacementApplies(ev) {
		e.parkCommanderZoneMove(ev)
		return ev, true
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
		// X is the {X} paid for the moving object, so an ETB replacement that
		// reads it (etbCounter's CounterNum$ X, e.g. Endless One / Walking
		// Ballista / Chalice of the Void) sees the value the player actually
		// chose. Move preserves X from the stack onto the permanent (events/
		// apply.go, the "hand/stack -> battlefield must NOT reset them"
		// comment), so o.X is the cast-time value here.
		X:          o.X,
		Remembered: []state.Target{{Obj: ev.Obj}},
		// Replaced names the object the replaced event (ev) was about, so a
		// ReplaceWith$ that says Defined$ ReplacedCard (the Rest in Peace / Dryad
		// Militant / Leyline of the Void shape: "exile it instead") can act on
		// exactly the card being kept out of the graveyard -- not the source that
		// owns the replacement.
		Replaced: ev.Obj}
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
		e.resolveReplacementWith(ctx, matchRepl.With)
		e.applyingReplacement = false
		return stored, true
	}

	// Anything else -- ReplacementResult$ absent or "Replaced", or any
	// other value -- keeps today's behaviour: the original event is
	// discarded and only ReplaceWith$'s own effect happens. Task 22's four
	// pins are exactly this shape (no ReplacementResult$ at all) and must
	// not move.
	e.applyingReplacement = true
	e.resolveReplacementWith(ctx, matchRepl.With)
	e.applyingReplacement = false
	return ev, true
}

// resolveReplacementWith runs a ReplaceWith$ effect with e.damaging set to
// the permanent that owns the replacement and then restores whatever it was
// beforehand (Task 15 fix round 1, Important I3). A ReplaceWith$ resolves
// inside emit, where the only write to e.damaging so far was around
// resolveTop's own resolution calls and damageStep's assignment loop -- so a
// replacement that emits damage normally inherited whichever value that outer
// context happened to hold: the combat attacker when the replaced event came
// from damageStep's loop, or 0 (no source) during ordinary turn structure.
// Either way the damage was attributed to the wrong thing, or to nothing.
// The reading chosen here is: the damage a replacement emits is dealt by the
// permanent whose R: line owns the replacement (ctx.Source) -- it is an
// effect of that permanent, exactly as resolveTop attributes a resolved
// ability's damage to its source -- and CR 609.7a asks for the source of the
// effect, which is the permanent granting the replacement. The PREVIOUS
// damaging is saved and restored (never zeroed) so an outer in-flight
// assignment keeps its own attribution once the replacement returns.
func (e *Engine) resolveReplacementWith(ctx *effects.Ctx, with *cards.SA) {
	saved := e.damaging
	e.damaging = ctx.Source
	effects.Resolve(e, ctx, with)
	e.damaging = saved
}

// replacementMatches implements R:Event$ Moved's own Origin$/Destination$/
// ValidCard$ parameters -- the same shape as zoneChangeMatches, for a
// replacement instead of a trigger.
func (e *Engine) replacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	if r.Event != "Moved" || ev.Kind != events.MoveZone {
		return false
	}
	// CR 611.3b/614.4: a static replacement only applies from one of its
	// declared active zones. Accept the comma-separated list grammar used by
	// other Forge zone parameters; the pinned corpus currently uses only
	// singleton ActiveZones values. Preserve replacements with no ActiveZones
	// parameter: the corpus does not thereby declare a zone, and
	// historically this engine has allowed those replacements from anywhere.
	//
	// A permanent's own entry replacement is active for the event that puts it
	// into the declared zone even though the source has not arrived there yet
	// (CR 614.12). Without the prospective ev.To check, ordinary "enters with"
	// replacements would disable themselves while their source is in hand or
	// on the stack.
	if active, ok := r.Params["ActiveZones"]; ok {
		o := e.G.Obj(source)
		currentlyActive := o != nil && zoneSpecContains(active, o.Zone)
		enteringActive := source == ev.Obj && zoneSpecContains(active, ev.To)
		if !currentlyActive && !enteringActive {
			return false
		}
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

func init() {
	// kw:etbCounter and kw:ETBReplacement are implemented wholly by the
	// machinery above: both are R:Event$ Moved replacements (expanded from a
	// K: line by cards/keywords.go) matched and applied here. Reading a card's
	// own tags is what a replacement registration means -- nothing elsewhere
	// in the tree registers them.
	effects.RegisterNonAPI("kw:etbCounter", "kw:ETBReplacement")
}

// cmdZoneMove is one parked commander zone change (CR 903.9, Task m32): the
// MoveZone event a commander is about to undergo, deferred until its owner
// decides whether to put it into the command zone instead. The answer
// re-emits the park as a real MoveZone -- to ZCommand on an accept, to the
// parked destination verbatim on a decline -- so the event log always
// carries the zone change that actually happened and a log-only replay
// reproduces it. Plain value data (an events.Event plus the moving object's
// id), so Clone copies the queue with one slice copy.
type cmdZoneMove struct {
	ev  events.Event
	obj state.ObjID
}

// commanderZoneReplacementApplies is CR 903.9's match predicate: a
// commander, in a Commander-format game, about to be put into its owner's
// graveyard, hand or library -- from ANYWHERE -- or about to be exiled from
// anywhere. The five points of the rule each live in exactly one place here
// (mutation guards, Task m32 brief item 7):
//
//   - FormatCommander gate: no format check, no mechanic. A Constructed
//     game never runs any of this.
//   - The four destinations: graveyard, hand, library, exile -- a
//     commander moving to the battlefield or the stack is not replaced.
//   - "From anywhere": ev.From is never consulted. The battlefield, the
//     stack, the graveyard, a hand a library or exile are all sources.
//   - Ownership, not control: the lookup is the owner's Commanders list,
//     not the controller's -- a commander stolen by an opponent goes to its
//     owner's command zone and its owner is asked.
func (e *Engine) commanderZoneReplacementApplies(ev events.Event) bool {
	if e.format != FormatCommander {
		return false
	}
	switch ev.To {
	case state.ZGraveyard, state.ZHand, state.ZLibrary, state.ZExile:
	default:
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || int(o.Owner) >= len(e.G.Players) {
		return false
	}
	for _, c := range e.G.Players[o.Owner].Commanders {
		if c == ev.Obj {
			return true
		}
	}
	return false
}

// parkCommanderZoneMove applies the CR 903.9 replacement to one matching
// zone change: the event is deferred (never logged, never applied -- the
// commander stays where it is) and its owner is asked whether to put the
// commander into the command zone instead. handleCmdZone emits the parked
// move for real when the answer lands.
//
// Dedup by object: while a commander's decision is outstanding the only
// engine work that can run is (a) the resolution-chain tail that already
// emitted the first park and (b) Submit's repeating state-based-action pass.
// A second park for the same commander is therefore either the SAME zone
// change being re-offered (the SBA pass re-finding a commander it already
// had tried -- the no-progress shape a replacement must not loop on, the
// stalledCastLimit lesson: the already-pending decision covers it, so the
// duplicate is dropped) or a second effect in the same chain that in the
// real rules would resolve AFTER the commander has already moved (its own
// CR 608.2b Origin$-driven recheck would then skip it, so dropping is the
// more-correct outcome, not merely safe). The move itself always happens
// exactly once, through the front decision.
//
// An owner who has left the game cannot exercise a "may" choice (CR 800.4a:
// a departed player makes no choices), so the unexercised choice is a
// decline: the original move happens unchanged, emitted here under
// applyingReplacement so the commander check that just matched cannot
// re-park it (CR 616.1, a replacement applies only once). This is reachable
// when removePermanents sweeps a Lost player's own commander to exile.
func (e *Engine) parkCommanderZoneMove(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return
	}
	owner := o.Owner
	if e.G.Players[owner].Lost {
		saved := e.applyingReplacement
		e.applyingReplacement = true
		e.emit(ev)
		e.applyingReplacement = saved
		return
	}
	for _, pm := range e.cmdZone {
		if pm.obj == ev.Obj {
			return
		}
	}
	e.cmdZone = append(e.cmdZone, cmdZoneMove{ev: ev, obj: ev.Obj})
	if e.pending == nil {
		e.askCommandZone(owner)
	}
}

// askCommandZone poses the CR 903.9 choice for the FRONT parked move to its
// owner, following askTriggerOptional's shape: Min == Max == 1 over two
// options, first the "change the outcome" one, then the "let it happen"
// one. Only the front of the queue is ever asked -- see handleCmdZone's
// resumption for how the queue hands from one decision to the next.
func (e *Engine) askCommandZone(owner state.PlayerID) {
	pm := e.cmdZone[0]
	name := "this commander"
	if o := e.G.Obj(pm.obj); o != nil && o.Face() != nil && o.Face().Name != "" {
		name = o.Face().Name
	}
	dest := pm.ev.To.String()
	into := "Put " + name + " into the command zone"
	d := &decision.Decision{Player: owner, Kind: decision.KCommanderZone, Min: 1, Max: 1,
		Prompt: name + " would go to the " + dest + ": put it into the command zone instead?",
		Source: pm.obj,
		Options: []decision.Option{
			{Index: 0, Kind: "command_zone", Label: into, Obj: pm.obj, Player: owner},
			{Index: 1, Kind: "leave", Label: "Let it go to the " + dest, Obj: pm.obj, Player: owner},
		}}
	e.ask(d)
}

// handleCmdZone applies an answered CR 903.9 decision: the front parked move
// is emitted for real -- to the command zone if the owner chose
// "command_zone", verbatim (the destination it was heading for) if they
// chose "leave" -- and, if more moves are parked, the next one's owner is
// asked. The de-park emit runs under applyingReplacement: the CR 903.9
// replacement has already applied to this zone change, and CR 616.1 lets a
// replacement effect apply only once, so the final move is never re-parked
// and never re-asked -- a decline therefore cannot spin the engine by
// re-offering the same choice, and the SBA pass that re-finds the commander
// after this answer degrades the same way it would for any other completed
// replacement (the dedup in parkCommanderZoneMove handled its in-flight
// copy).
//
// The owner's choice is in the log as the Intents entry plus the
// DecisionAsk/DecisionMade events every decision emits; the outcome is the
// MoveZone event below, so a log-only replay reproduces both branches from
// the log alone. An answer with no parked move (only reachable from a
// hand-built decision -- every real ask parks one) degrades to a Note, the
// same totality stance as handleModes.
func (e *Engine) handleCmdZone(d *decision.Decision, in decision.Intent) {
	if len(e.cmdZone) == 0 {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "commander-zone decision answered with no move parked"})
		return
	}
	pm := e.cmdZone[0]
	e.cmdZone = e.cmdZone[1:]
	to := pm.ev.To
	if opts := d.Chosen(in); len(opts) == 1 && opts[0].Kind == "command_zone" {
		to = state.ZCommand
	}
	saved := e.applyingReplacement
	e.applyingReplacement = true
	e.emit(events.Event{Kind: events.MoveZone, Obj: pm.ev.Obj, From: pm.ev.From,
		To: to, Player: pm.ev.Player, Text: pm.ev.Text})
	e.applyingReplacement = saved
	if len(e.cmdZone) > 0 && e.pending == nil {
		// More commanders were parked in the same burst (a board wipe, a
		// multiple-SBA pass): hand the front of the queue to its owner the
		// same way handleTriggerOptional resumes its own drain. The queue is
		// empty exactly when the previous answer WAS the front, so popping
		// above and asking here keeps every decision aligned with the move
		// it resolves.
		if o := e.G.Obj(e.cmdZone[0].obj); o != nil && int(o.Owner) < len(e.G.Players) {
			e.askCommandZone(o.Owner)
		}
	}
}
