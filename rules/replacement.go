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
	// Collect EVERY replacement effect this MoveZone event matches, in
	// forEachObject's deterministic scan order, rather than the single first
	// match the M1 build took.
	var matches []replMatch
	e.forEachObject(func(id state.ObjID) {
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
				matches = append(matches, replMatch{id: id, repl: &f.Repls[i]})
			}
		}
	})
	if len(matches) == 0 {
		return ev, false
	}

	// CR 616.1: if two or more replacement effects would modify the way this
	// event affects an object, each gets one opportunity to apply and the
	// affected player chooses the order. Two shapes follow.
	//
	//   - All matches are "Updated" (Forge's "the event still happens,
	//     augmented" idiom, e.g. an "enters tapped" plus an "enters with
	//     counters"). The original event is emitted once and each With is
	//     resolved in turn, so the object finishes with BOTH characteristics
	//     set (CR 616.1f) instead of whichever scan happened to reach first.
	//     Every ordering lands the same result for that commute shape, so no
	//     player order choice is posed -- the CR 616.1 choice is about order
	//     that can CHANGE the result, and a pure-augment competition never
	//     reorders a destination, so it would be a decision nobody answers
	//     differently.
	//
	//   - Otherwise at least one "Replaced"/destination-changing replacement
	//     competes (Rest in Peace's exile vs. Darksteel Colossus's shuffle):
	//     the affected controller must choose BEFORE anything relocates, so
	//     the event is parked and a KReplacement choice is posed (see
	//     poseReplacementChoice / handleReplacement).
	if len(matches) == 1 {
		return e.applyReplacement(ev, matches[0])
	}
	allUpdated := true
	for _, m := range matches {
		if m.repl.Params["ReplacementResult"] != "Updated" {
			allUpdated = false
			break
		}
	}
	if allUpdated {
		return e.composeUpdatedReplacements(ev, matches)
	}
	// Not all "Updated": at least one destination-changing ("Replaced")
	// replacement competes, so the affected controller must choose BEFORE
	// anything relocates (CR 616.1). A controller who has left the game makes
	// no choices (CR 800.4a), so for them the first replacement that has a
	// body applies deterministically rather than stalling the engine.
	if ea := e.G.Obj(ev.Obj); ea != nil {
		p := ea.Controller
		if int(p) < len(e.G.Players) && !e.G.Players[p].Lost {
			e.poseReplacementChoice(ev, matches)
			return ev, true
		}
	}
	for _, m := range matches {
		if m.repl.With != nil {
			return e.applyReplacement(ev, m)
		}
	}
	return ev, false
}

// replMatch is one replacement effect the engine found applicable to a
// MoveZone event: its owning source permanent and the R: line on that
// permanent's face. A plain value, cloned by copy.
type replMatch struct {
	id   state.ObjID
	repl *cards.Repl
}

// replCtx builds the effects.Ctx a replacement's ReplaceWith$ resolves
// under: source is the permanent whose R: line owns the replacement, X is
// the {X} paid for the moving object, Remembered and Replaced both name the
// object the replaced event was about (so a Defined$ ReplacedCard /
// Remembered$Amount body finds its subject), and the SVar table comes from
// the source's face. This is exactly the context the single-match path built
// inline for M1; it is factored out so the composition and order-choice
// paths reuse it.
func (e *Engine) replCtx(m replMatch, ev events.Event) *effects.Ctx {
	o := e.G.Obj(m.id)
	if o == nil {
		return &effects.Ctx{Source: m.id}
	}
	ctx := &effects.Ctx{Source: m.id, Controller: o.Controller,
		// X is the {X} paid for the moving object, so an ETB replacement that
		// reads it (etbCounter's CounterNum$ X, e.g. Endless One / Walking
		// Ballista / Chalice of the Void) sees the value the player actually
		// chose. Move preserves X from the stack onto the permanent (events/
		// apply.go, the "hand/stack -> battlefield must NOT reset them"
		// comment), so o.X is the cast-time value here.
		X:          o.X,
		Remembered: []state.Target{{Obj: ev.Obj}},
		// Replaced names the object the replaced event (ev) was about, so a
		// ReplaceWith$ that says Defined$ ReplacedCard (the Rest in Peace /
		// Dryad Militant / Leyline of the Void shape: "exile it instead") can
		// act on exactly the card being kept out of the graveyard -- not the
		// source that owns the replacement.
		Replaced: ev.Obj}
	if f := o.Face(); f != nil {
		effects.SetSVars(ctx, f.SVars)
	}
	return ctx
}

// runReplaceWith resolves one ReplaceWith$ body inside the replacement
// re-entrancy guard (CR 616.1, a replacement applies once), recording the
// replaced object for the body's Defined$/Remembered$ reads and restoring
// both the guard and that record afterward so an outer replacement keeps its
// own state.
func (e *Engine) runReplaceWith(ctx *effects.Ctx, replaced state.ObjID, with *cards.SA) {
	savedRepl := e.replReplaced
	e.applyingReplacement = true
	e.replReplaced = replaced
	e.resolveReplacementWith(ctx, with)
	e.replReplaced = savedRepl
	e.applyingReplacement = false
}

// applyReplacement applies the ONE chosen replacement to a MoveZone event,
// the exact behaviour the M1 build had for its single matching replacement.
// An "Updated" match emits the original event, fires triggers (in the M1
// order: before the With resolves -- the I-3 caveat stands), then resolves
// the With; anything else ("Replaced", or no ReplacementResult$ at all --
// Task 22's four pins must keep reading as a full replace) discards the
// original and only the With's own effect happens. Returns the event to log
// (stored for Updated, ev for Replaced) and handled=true.
func (e *Engine) applyReplacement(ev events.Event, m replMatch) (events.Event, bool) {
	if m.repl.With == nil {
		return ev, false
	}
	ctx := e.replCtx(m, ev)
	if m.repl.Params["ReplacementResult"] == "Updated" {
		// Review finding M-6: an exact case-sensitive "Updated" compare is
		// deliberate (the corpus spells it one way). Apply the ORIGINAL event
		// first (not routing back through e.emit, which is what keeps this
		// from re-running replacement matching on the event it just
		// matched), fire triggers (an ETB trigger watching this Move fires
		// exactly as it would for an unreplaced entry), THEN resolve the With
		// so a Tap lands on an object already in its new zone (an object
		// still on the stack is a no-op to effTap).
		stored := events.Emit(e.G, e.L, ev)
		e.checkTriggers(stored, nil)
		e.runReplaceWith(ctx, ev.Obj, m.repl.With)
		return stored, true
	}
	e.runReplaceWith(ctx, ev.Obj, m.repl.With)
	return ev, true
}

// composeUpdatedReplacements applies every applicable "Updated" replacement
// to one MoveZone event: the original event is emitted and triggers fire
// once, then each With is resolved in the deterministic scan order (CR
// 616.1f, each applicable replacement gets one opportunity). This is the
// composition a competing set of entry replacements needs -- a permanent that
// both "enters tapped" (Blind Obedience) and "enters with three +1/+1
// counters" (Triskelion) finishes BOTH attrs set, not whichever the scan
// reached first.
func (e *Engine) composeUpdatedReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored, nil)
	for _, m := range matches {
		if m.repl.With == nil {
			continue
		}
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With)
	}
	return stored, true
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
// ValidCard$/ValidLKI$ parameters -- the same shape as zoneChangeMatches, for
// a replacement instead of a trigger.
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
		if !effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source) {
			return false
		}
	}
	// CR 603.10/Forge ValidLKI: a look-back-in-time gate on the moving
	// object, evaluated against it as it is right before the move applies --
	// which for a replacement is its live state, since a replacement runs
	// ahead of the Move it intercepts. The same filter grammar as ValidCard,
	// evaluated with MatchesObjectCtx (the LKI-form matcher) so the object is
	// matched by value. Unknown predicates fail closed (filter.go's contract),
	// so a gate this build cannot evaluate -- e.g. Forge's may-play-from-
	// graveyard provenance spec "Card.CastSa Spell.MayPlaySource" on the
	// Eelectrocute/Glimpse the Cosmos family, where the engine has no
	// cast-from-graveyard path at all -- NEVER admits the replacement: the
	// conservative direction, since an ordinary hand-origin cast of such a
	// card must still finish in the graveyard.
	if v, ok := r.Params["ValidLKI"]; ok {
		mo := e.G.Obj(ev.Obj)
		if mo == nil || !effects.MatchesObjectCtx(e.G, v, mo, effects.SpecContext{
			You: e.controllerOf(source), Source: source}) {
			return false
		}
	}
	return true
}

// replChoice is one CR 616.1 order-selection suspension: the MoveZone event
// more than one replacement would modify, parked (never emitted, never
// applied -- the moving object stays put) along with the competing
// replacements, until the affected controller picks the order. The answer
// applies the chosen replacement for real. Plain value data (an events.Event
// plus a []replMatch whose *cards.Repl pointers are shared immutable corpus
// data), so Clone copies the queue with one slice copy, the same class as
// cmdZone.
type replChoice struct {
	ev    events.Event
	cands []replMatch
}

// poseReplacementChoice starts a CR 616.1 order-selection suspension: the
// competing event is parked and the affected controller -- the controller of
// the moving object, CR 616.1's "affected player" -- is asked which
// replacement applies first. Mirrors commander-zone parking: the ask is posed
// only when no other decision is already pending (the caller has already
// ruled out a departed controller, which makes no choices under CR 800.4a).
func (e *Engine) poseReplacementChoice(ev events.Event, matches []replMatch) {
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return
	}
	p := o.Controller
	if int(p) >= len(e.G.Players) {
		return
	}
	e.replChoices = append(e.replChoices, replChoice{ev: ev, cands: matches})
	if e.pending == nil {
		e.askReplacementChoice(p)
	}
}

// askReplacementChoice poses the CR 616.1 order choice for the FRONT parked
// competition to the affected controller: Min == Max == 1 over one option
// per competing replacement, in the deterministic scan order the engine found
// them in (the order the player reorders, never a coincidence of map
// iteration). Only the front of the queue is ever asked -- see
// handleReplacement's resumption for how the queue hands from one choice to
// the next.
func (e *Engine) askReplacementChoice(p state.PlayerID) {
	rc := e.replChoices[0]
	name := "this object"
	if o := e.G.Obj(rc.ev.Obj); o != nil && o.Face() != nil && o.Face().Name != "" {
		name = o.Face().Name
	}
	d := &decision.Decision{Player: p, Kind: decision.KReplacement, Min: 1, Max: 1,
		Prompt: "Several replacement effects would change how " + name + " moves: choose the order they apply.",
		Source: rc.ev.Obj}
	for i, c := range rc.cands {
		label := "a replacement"
		if so := e.G.Obj(c.id); so != nil && so.Face() != nil && so.Face().Name != "" {
			label = "Apply " + so.Face().Name + "'s replacement"
		} else if dsc := c.repl.Params["Description"]; dsc != "" {
			label = "Apply: " + dsc
		}
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "replacement", Obj: c.id, Label: label})
	}
	e.ask(d)
}

// handleReplacement applies an answered CR 616.1 order choice: the front
// parked competition's chosen replacement is applied for real -- the SAME
// applyReplacement a lone matching replacement would run -- and, if more
// competitions are parked, the next one's controller is asked. Because the
// chosen replacement is the one the player decided applies FIRST, and a
// "Replaced" replacement discards the modified event (so the remaining
// replacements then see a destination the original no longer matches), one
// application completes the relocation for the competing shape. The choice
// is in the log as the Intents entry plus the DecisionAsk/DecisionMade
// events every decision emits; the relocation is the MoveZone (or absence of
// it) the applied replacement emits, so a log-only replay reproduces both
// branches. An answer with no parked competition is only reachable from a
// hand-built decision and degrades to a Note, the same totality stance as
// handleCmdZone.
func (e *Engine) handleReplacement(d *decision.Decision, in decision.Intent) {
	if len(e.replChoices) == 0 {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement-order decision answered with no competition parked"})
		return
	}
	rc := e.replChoices[0]
	e.replChoices = e.replChoices[1:]
	chosen := d.Chosen(in)
	if len(chosen) == 0 || chosen[0].Index < 0 || chosen[0].Index >= len(rc.cands) {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement-order answer out of range"})
		return
	}
	e.applyReplacement(rc.ev, rc.cands[chosen[0].Index])
	if len(e.replChoices) > 0 && e.pending == nil {
		if o := e.G.Obj(e.replChoices[0].ev.Obj); o != nil && int(o.Controller) < len(e.G.Players) {
			e.askReplacementChoice(o.Controller)
		}
	}
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
// re-park it (CR 616.1, a replacement applies only once).
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
