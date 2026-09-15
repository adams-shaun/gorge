// replacement.go applies R: line replacement effects ahead of an event's
// own logging, from engine.go's emit. applyReplacements finds the single
// best-fitting match via forEachObject's deterministic scan and
// replacementMatches's predicate, then either runs ReplaceWith$ in place of
// the original event or -- for ReplacementResult$ Updated -- runs the
// original event first and ReplaceWith$ after.
package rules

import (
	"math"
	"strconv"
	"strings"

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
	if ev.Kind == events.MoveZone && e.commanderZoneReplacementApplies(ev) {
		e.parkCommanderZoneMove(ev)
		return ev, true
	}
	// Collect EVERY replacement effect this event matches, in
	// forEachObject's deterministic scan order, rather than the single first
	// match the M1 build took.
	var matches []replMatch
	// Effect-created replacements (Blood of the Martyr and the broader
	// ReplacementEffects$ family) are active independently of their source's
	// current zone. active() enforces the Effect duration; reconstruct the
	// Forge R: body into the same replMatch path used by printed replacements
	// so filters, ordering and replacement context cannot drift.
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "" || ce.ReplacementBody == "" {
			continue
		}
		if with := replacementBodySA(ce.ReplacementBody); with != nil {
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: with}
			if e.replacementMatches(*r, ce.Source, ev) {
				matches = append(matches, replMatch{id: ce.Source, repl: r,
					key: "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))})
			}
		}
	}
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
	if ev.Kind != events.MoveZone {
		if ev.Kind == events.Damage {
			matches = e.applicableDamageReplacements(ev, matches)
			if len(matches) == 0 {
				return ev, false
			}
			if len(matches) > 1 || hasOptionalReplacement(matches) {
				if p, ok := e.damageAffectedPlayer(ev); ok && !e.G.Players[p].Lost {
					e.poseDamageReplacementChoice(ev, matches, p)
					return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
						Text: "damage awaiting replacement-order choice"}, true
				}
			}
		}
		return e.applyNonMoveReplacements(ev, matches)
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

// applyNonMoveReplacements applies a lone damage replacement, or the
// deterministic fallback used when the affected player has left the game.
// Competing replacements for a live affected player are parked and ordered by
// KReplacement instead.
func (e *Engine) applyNonMoveReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	for _, m := range matches {
		// CR 616.1e: after each modification, applicability is checked again
		// against the changed event (not the original amount).
		if !e.replacementMatches(*m.repl, m.id, ev) {
			continue
		}
		if ev.Kind == events.Damage && m.repl.Params["Prevent"] == "True" {
			if !e.cantPreventDamage(e.damaging, ev.Obj) {
				return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
					Text: "damage prevented by replacement effect"}, true
			}
			continue
		}
		if m.repl.With == nil {
			// Counter's Layer$ CantHappen shape has no ReplaceWith$: stopping
			// the event is its complete replacement.
			return ev, true
		}
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, &ev)
		if m.repl.With.API == "ReplaceEffect" {
			// The body rewrote the held amount (changed) or could not resolve
			// its value and left it alone; either way the event stands and the
			// next modifier applies to the result.
			continue
		}
		// A body of another API (DB$ DealDamage, DB$ RemoveCounters, ...)
		// supplied its own outcome; its emissions replace the original event.
		return ev, true
	}
	return ev, false
}

func (e *Engine) applicableDamageReplacements(ev events.Event, matches []replMatch) []replMatch {
	out := matches[:0]
	for _, m := range matches {
		if !e.replacementMatches(*m.repl, m.id, ev) {
			continue
		}
		if m.repl.Params["Prevent"] == "True" && e.cantPreventDamage(e.damaging, ev.Obj) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func (e *Engine) damageAffectedPlayer(ev events.Event) (state.PlayerID, bool) {
	if ev.Obj == 0 {
		return ev.Player, int(ev.Player) < len(e.G.Players)
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || int(o.Controller) >= len(e.G.Players) {
		return 0, false
	}
	return o.Controller, true
}

// replMatch is one replacement effect the engine found applicable to an
// event: its owning source permanent and the R: line on that permanent's
// face. A plain value, cloned by copy.
type replMatch struct {
	id   state.ObjID
	repl *cards.Repl
	// key identifies an Effect-created replacement across active() rebuilds.
	// Printed replacement pointers are immutable face entries and need no key.
	key string
}

func hasOptionalReplacement(matches []replMatch) bool {
	for _, m := range matches {
		if strings.EqualFold(m.repl.Params["Optional"], "True") {
			return true
		}
	}
	return false
}

// replacementBodySA turns the body retained by an Effect-created replacement
// into the same immutable SA shape cards.Parse builds for a printed R: line.
// It deliberately shares the ordinary `Kind$ API | Key$ Value` grammar rather
// than recognizing Blood of the Martyr by name.
func replacementBodySA(body string) *cards.SA {
	parts := strings.Split(body, "|")
	if len(parts) == 0 {
		return nil
	}
	head := strings.TrimSpace(parts[0])
	kind, api, ok := strings.Cut(head, "$")
	if !ok || strings.TrimSpace(api) == "" {
		return nil
	}
	sa := &cards.SA{Kind: strings.TrimSpace(kind), API: strings.TrimSpace(api), Params: make(map[string]string)}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "$")
		if ok && strings.TrimSpace(key) != "" {
			sa.Params[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return sa
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
	target := state.Target{Obj: ev.Obj}
	if ev.Kind == events.Damage && ev.Obj == 0 {
		target = state.Target{Player: ev.Player, IsPlayer: true}
	}
	if o == nil {
		return &effects.Ctx{Source: m.id, ReplacementTarget: target,
			ReplacementSource: e.protectionSource(e.damaging), ReplacementAmount: ev.Amount}
	}
	ctx := &effects.Ctx{Source: m.id, Controller: o.Controller,
		ReplacementTarget: target, ReplacementSource: e.protectionSource(e.damaging),
		ReplacementAmount: ev.Amount,
		// X is the {X} paid for the moving object, so an ETB replacement that
		// reads it (etbCounter's CounterNum$ X, e.g. Endless One / Walking
		// Ballista / Chalice of the Void) sees the value the player actually
		// chose. Move preserves X from the stack onto the permanent (events/
		// apply.go, the "hand/stack -> battlefield must NOT reset them"
		// comment), so o.X is the cast-time value here.
		X:          o.X,
		Remembered: []state.Target{target},
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
func (e *Engine) runReplaceWith(ctx *effects.Ctx, replaced state.ObjID, with *cards.SA, ev *events.Event) {
	savedRepl, savedEvent, savedSource := e.replReplaced, e.replacingEvent, e.replacingSource
	e.applyingReplacement = true
	e.replReplaced, e.replacingEvent, e.replacingSource = replaced, ev, ctx.Source
	e.resolveReplacementWith(ctx, with)
	e.replReplaced, e.replacingEvent, e.replacingSource = savedRepl, savedEvent, savedSource
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
		departing, link := e.captureSourceLifelinkLKI(ev)
		stored := events.Emit(e.G, e.L, ev)
		e.checkTriggers(stored, nil)
		e.finishSourceLifelinkLKI(ev, departing, link)
		e.runReplaceWith(ctx, ev.Obj, m.repl.With, nil)
		return stored, true
	}
	e.runReplaceWith(ctx, ev.Obj, m.repl.With, nil)
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
	departing, link := e.captureSourceLifelinkLKI(ev)
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored, nil)
	e.finishSourceLifelinkLKI(ev, departing, link)
	for _, m := range matches {
		if m.repl.With == nil {
			continue
		}
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, nil)
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
	// ReplaceEffect rewrites the held event and must retain e.damaging as the
	// ORIGINAL damage source (Affected$ ReplacedSourceController needs it).
	// A body that emits its own damage still attributes that new event to the
	// permanent owning the replacement.
	if with.API == "ReplaceEffect" {
		effects.Resolve(e, ctx, with)
		return
	}
	saved := e.damaging
	e.damaging = ctx.Source
	effects.Resolve(e, ctx, with)
	e.damaging = saved
}

// replacementMatches implements R:Event$ Moved's own Origin$/Destination$/
// ValidCard$/ValidLKI$ parameters -- the same shape as zoneChangeMatches, for
// a replacement instead of a trigger.
func (e *Engine) replacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	switch r.Event {
	case "Moved":
		if ev.Kind != events.MoveZone {
			return false
		}
	case "DamageDone":
		if ev.Kind != events.Damage || !e.damageReplacementMatches(r, source, ev) {
			return false
		}
	default:
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
	if ev.Kind == events.MoveZone {
		if o, ok := r.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
			return false
		}
		if d, ok := r.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
			return false
		}
	}
	if v, ok := r.Params["ValidCard"]; ok {
		if !effects.MatchesSpecFrom(e.G, v, ev.Obj, e.controllerOf(source), source) {
			return false
		}
	}
	if !e.replacementConditionHolds(r.Params, source) {
		return false
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
	if ev.Kind == events.MoveZone {
		if v, ok := r.Params["ValidLKI"]; ok {
			mo := e.G.Obj(ev.Obj)
			if mo == nil || !effects.MatchesObjectCtx(e.G, v, mo, effects.SpecContext{
				You: e.controllerOf(source), Source: source}) {
				return false
			}
		}
	}
	return true
}

// damageReplacementMatches applies the damage-specific R: filters before the
// common active-zone gate: source and target are the actual damage source and
// recipient, and IsCombat$/DamageAmount$ describe this in-flight event.
func (e *Engine) damageReplacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	ctrl := e.controllerOf(source)
	if v := r.Params["ValidCause"]; v != "" && !e.replacementCauseMatches(v, source, e.damaging) {
		return false
	}
	if v := r.Params["ValidSource"]; v != "" &&
		(e.damaging == 0 || !effects.MatchesSpecFrom(e.G, v, e.damaging, ctrl, source)) {
		return false
	}
	if v := r.Params["ValidTarget"]; v != "" {
		if ev.Obj != 0 {
			if !effects.MatchesSpecFrom(e.G, v, ev.Obj, ctrl, source) {
				return false
			}
		} else if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	if combat := strings.TrimSpace(r.Params["IsCombat"]); combat != "" &&
		((strings.EqualFold(combat, "True") && !e.combatDamaging) ||
			(strings.EqualFold(combat, "False") && e.combatDamaging)) {
		return false
	}
	return e.replacementAmountMatches(r.Params["DamageAmount"], ev.Amount, e.replCtx(replMatch{id: source, repl: &r}, ev))
}

// replacementAmountMatches understands Forge's comparison shorthand such as
// LTX (Ojer Axonil). Its RHS is resolved in the replacement source's context.
func (e *Engine) replacementAmountMatches(spec string, amount int32, c *effects.Ctx) bool {
	if spec == "" {
		return true
	}
	for _, op := range []string{"GE", "GT", "LE", "LT", "EQ"} {
		if rhs, ok := strings.CutPrefix(spec, op); ok {
			v := effects.Num(e, c, &cards.SA{Params: map[string]string{"N": rhs}}, "N", 0)
			switch op {
			case "GE":
				return amount >= v
			case "GT":
				return amount > v
			case "LE":
				return amount <= v
			case "LT":
				return amount < v
			case "EQ":
				return amount == v
			}
		}
	}
	return false
}

// ReplaceEvent implements effects.Host. It rewrites the amount of the Damage
// event currently being replaced -- the one held in e.replacingEvent -- from
// either a resolved numeric value (a literal, or a Count$/SVar expression
// effects.Num already evaluated in the replacement source's context) or a
// ReplaceCount$ body, whose base is the HELD event's own amount and which
// only the host reading the in-flight event can resolve. Anything else -- an
// unresolvable value, an unknown field, a non-Damage event -- leaves the held
// event untouched: an amount replacement that cannot be computed is closer to
// the card than one that erases the damage or discards the event.
func (e *Engine) ReplaceEvent(name, raw string, resolved int32) {
	ev := e.replacingEvent
	if ev == nil || ev.Kind != events.Damage {
		return
	}
	if body, ok := strings.CutPrefix(raw, "ReplaceCount$"); ok {
		field, op, hasOp := strings.Cut(body, "/")
		if (field != "DamageAmount" && field != "Amount") || !hasOp {
			return
		}
		ev.Amount = replCountOp(ev.Amount, op)
		return
	}
	if (name == "DamageAmount" || name == "Amount") && resolved > 0 {
		ev.Amount = resolved
		return
	}
	if name != "Affected" {
		return
	}
	switch raw {
	case "You":
		ev.Obj, ev.Player = 0, e.controllerOf(e.replacingSource)
	case "Self":
		ev.Obj, ev.Player = e.replacingSource, 0
	case "Enchanted", "Equipped":
		if source := e.G.Obj(e.replacingSource); source != nil && source.AttachedTo != 0 {
			ev.Obj, ev.Player = source.AttachedTo, 0
		}
	case "ReplacedSourceController":
		if source := e.G.Obj(e.damaging); source != nil {
			ev.Obj, ev.Player = 0, source.Controller
		}
	case "ReplacedTargetController":
		if target := e.G.Obj(ev.Obj); target != nil {
			ev.Obj, ev.Player = 0, target.Controller
		}
	}
}

// replCountOp applies Forge's ReplaceCount$ arithmetic to a base amount: the
// corpus carries Twice (Bloodletter of Aclazotz), Thrice (Fiery Emancipation)
// and Plus.N (Torture Pit), with the rest of applyCountOp's op vocabulary
// implemented for the class rather than only the seen three. An op this
// builder does not parse returns the base unchanged.
func replCountOp(base int32, op string) int32 {
	v := int64(base)
	switch {
	case strings.HasPrefix(op, "Plus."):
		if x, err := strconv.Atoi(strings.TrimPrefix(op, "Plus.")); err == nil {
			v += int64(x)
		}
	case strings.HasPrefix(op, "Minus."):
		if x, err := strconv.Atoi(strings.TrimPrefix(op, "Minus.")); err == nil {
			v -= int64(x)
		}
	case strings.HasPrefix(op, "Times."):
		if x, err := strconv.Atoi(strings.TrimPrefix(op, "Times.")); err == nil {
			v *= int64(x)
		}
	case op == "Twice":
		v *= 2
	case op == "Thrice":
		v *= 3
	case op == "HalfDown":
		v /= 2
	case op == "HalfUp":
		v = (v + 1) / 2
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < 0 {
		return 0
	}
	return int32(v)
}

// replacementConditionHolds evaluates the condition gates shared by damage,
// counter and CantPreventDamage text. Unknown condition shapes fail closed:
// an inactive conditional replacement must never be widened into an
// unconditional one.
func (e *Engine) replacementConditionHolds(params map[string]string, source state.ObjID) bool {
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	ctrl := o.Controller
	if strings.EqualFold(params["PlayerTurn"], "True") && e.G.Active != ctrl {
		return false
	}
	if strings.EqualFold(params["Hellbent"], "True") && len(e.G.Zone(state.ZHand, ctrl)) != 0 {
		return false
	}
	if strings.EqualFold(params["Revolt"], "True") && !e.revoltThisTurn(ctrl) {
		return false
	}
	if strings.EqualFold(params["Delirium"], "True") && e.graveyardCardTypeCount(ctrl) < 4 {
		return false
	}
	if _, ok := params["CheckDefinedPlayer"]; ok {
		// The only corpus shape is You.isMonarch. Monarch state is not yet
		// represented, so fail closed instead of preventing damage always.
		return false
	}
	if spec, ok := params["IsPresent"]; ok {
		zone := state.ZBattlefield
		if z := params["PresentZone"]; z != "" {
			zone = effects.ParseZone(z)
		}
		n := e.countPresentInZone(spec, source, ctrl, zone, params["PresentDefined"])
		cmp := params["PresentCompare"]
		if cmp == "" {
			cmp = "GE1"
		}
		if !comparePresent(n, cmp) {
			return false
		}
	}
	if check, ok := params["CheckSVar"]; ok {
		n := e.replacementCheckValue(source, check)
		if cmp := params["SVarCompare"]; cmp != "" {
			op, rhs, valid := splitCompare(strings.TrimSpace(cmp))
			if !valid || !applyCompare(int(n), op, rhs) {
				return false
			}
		} else if n == 0 {
			return false
		}
	}
	return true
}

func (e *Engine) replacementCheckValue(source state.ObjID, check string) int32 {
	o := e.G.Obj(source)
	if o == nil {
		return 0
	}
	ctx := e.replCtx(replMatch{id: source}, events.Event{})
	if check == "X" {
		return o.X
	}
	body := check
	if ctx.SVars != nil {
		if v, ok := ctx.SVars[check]; ok {
			body = v
		}
	}
	switch body {
	case "Count$Party":
		roles := map[string]bool{}
		for _, id := range e.G.Zone(state.ZBattlefield, o.Controller) {
			if f := e.G.Obj(id).Face(); f != nil {
				for _, typ := range f.Types {
					switch typ {
					case "Cleric", "Rogue", "Warrior", "Wizard":
						roles[typ] = true
					}
				}
			}
		}
		return int32(len(roles))
	case "Count$Valid Permanent.YouCtrl$Colors":
		colors := ""
		for _, id := range e.G.Zone(state.ZBattlefield, o.Controller) {
			colors += effects.ColorsOf(e.G.Obj(id))
		}
		var n int32
		for _, c := range "WUBRG" {
			if strings.ContainsRune(colors, c) {
				n++
			}
		}
		return n
	case "Count$Presence_Dragon.1.0":
		for _, id := range e.G.Zone(state.ZBattlefield, o.Controller) {
			if faceHasType(e.G.Obj(id), "Dragon") {
				return 1
			}
		}
		return 0
	}
	return effects.EvalCount(e, ctx, body)
}

func (e *Engine) countPresentInZone(spec string, source state.ObjID, you state.PlayerID, zone state.Zone, defined string) int {
	if defined == "Self" {
		o := e.G.Obj(source)
		if o == nil || o.Zone != zone {
			return 0
		}
		if spec == "Card.equipping" {
			if o.AttachedTo != 0 {
				return 1
			}
			return 0
		}
		if effects.MatchesSpecFrom(e.G, spec, source, you, source) {
			return 1
		}
		return 0
	}
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o != nil && o.Zone == zone && effects.MatchesSpecFrom(e.G, spec, id, you, source) {
			n++
		}
	})
	return n
}

func (e *Engine) revoltThisTurn(controller state.PlayerID) bool {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.MoveZone && ev.From == state.ZBattlefield {
			// Move preserves the object's pre-move controller outside the
			// battlefield, so this is the controller at the moment it left —
			// exactly Revolt's "a permanent you controlled" test (not owner).
			if o := e.G.Obj(ev.Obj); o != nil && o.Controller == controller {
				return true
			}
		}
	}
	return false
}

func (e *Engine) graveyardCardTypeCount(controller state.PlayerID) int {
	seen := map[string]bool{}
	for _, id := range e.G.Zone(state.ZGraveyard, controller) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			for _, typ := range o.Face().Types {
				switch typ {
				case "Artifact", "Battle", "Creature", "Enchantment", "Instant", "Kindred", "Land", "Planeswalker", "Sorcery":
					seen[typ] = true
				}
			}
		}
	}
	return len(seen)
}

func faceHasType(o *state.Object, typ string) bool {
	if o == nil || o.Face() == nil {
		return false
	}
	for _, got := range o.Face().Types {
		if got == typ {
			return true
		}
	}
	return false
}

func (e *Engine) replacementCauseMatches(spec string, replacementSource, cause state.ObjID) bool {
	o := e.G.Obj(cause)
	if o == nil {
		return false
	}
	kind, quals, _ := strings.Cut(strings.TrimSpace(spec), ".")
	switch kind {
	case "Spell":
		if o.Ability != nil {
			return false
		}
	case "SpellAbility":
		// Both spell cards and minted ability objects qualify.
	default:
		return false
	}
	if quals == "" {
		return true
	}
	if strings.HasPrefix(quals, "IsTargeting Self") {
		for _, t := range o.Targets {
			if !t.IsPlayer && t.Obj == replacementSource {
				return true
			}
		}
		return false
	}
	switch quals {
	case "YouCtrl":
		return o.Controller == e.controllerOf(replacementSource)
	case "OppCtrl", "YouDontCtrl":
		return o.Controller != e.controllerOf(replacementSource)
	}
	return false
}

// CounterAllowed implements effects.Host. A Counter event is the attempted
// removal of a stack object, not a CounterChange event, so it is checked at
// Counter's sole stack-removal path before the MoveZone is emitted.
func (e *Engine) CounterAllowed(target, cause state.ObjID) bool {
	matches := e.counterReplacementMatchesAll(target, cause)
	switch len(matches) {
	case 0:
		return true
	case 1:
		e.applyCounterReplacement(target, matches[0])
		return false
	default:
		t := e.G.Obj(target)
		if t == nil || int(t.Controller) >= len(e.G.Players) || e.G.Players[t.Controller].Lost {
			e.applyCounterReplacement(target, matches[0])
			return false
		}
		e.replChoices = append(e.replChoices, replChoice{
			ev: events.Event{Obj: target}, cands: matches, before: e.triggerBefore,
			player: t.Controller, counter: true, cause: cause,
		})
		if e.pending == nil {
			e.askReplacementChoice(t.Controller)
		}
		return false
	}
}

func (e *Engine) counterReplacementMatchesAll(target, cause state.ObjID) []replMatch {
	var matches []replMatch
	e.forEachObject(func(source state.ObjID) {
		o := e.G.Obj(source)
		if o == nil || o.Face() == nil {
			return
		}
		for i := range o.Face().Repls {
			r := &o.Face().Repls[i]
			if r.Event == "Counter" && e.counterReplacementMatches(*r, source, target, cause) {
				matches = append(matches, replMatch{id: source, repl: r})
			}
		}
	})
	return matches
}

func (e *Engine) applyCounterReplacement(target state.ObjID, m replMatch) {
	if m.repl.With != nil {
		e.runReplaceWith(e.replCtx(m, events.Event{Obj: target}), target, m.repl.With, nil)
	}
}

func (e *Engine) applyChosenCounterReplacement(rc replChoice, selected int) {
	m := rc.cands[selected]
	// CR 616.1e: applicability is checked against the event as it exists when
	// the answer is applied. No state can normally change while the choice is
	// pending, but recomputing keeps this path correct for released/departed
	// decisions and mirrors damage replacement ordering.
	if !e.counterReplacementMatches(*m.repl, m.id, rc.ev.Obj, rc.cause) {
		matches := e.counterReplacementMatchesAll(rc.ev.Obj, rc.cause)
		if len(matches) == 0 {
			return
		}
		m = matches[0]
	}
	e.applyCounterReplacement(rc.ev.Obj, m)
}

func (e *Engine) counterReplacementMatches(r cards.Repl, source, target, cause state.ObjID) bool {
	o := e.G.Obj(source)
	t := e.G.Obj(target)
	if o == nil || t == nil || t.Zone != state.ZStack {
		return false
	}
	if active := r.Params["ActiveZones"]; active != "" && !zoneSpecContains(active, o.Zone) {
		return false
	}
	if v := r.Params["ValidCard"]; v != "" &&
		!effects.MatchesSpecFrom(e.G, v, target, o.Controller, source) {
		return false
	}
	if v := r.Params["ValidCause"]; v != "" && !e.replacementCauseMatches(v, source, cause) {
		return false
	}
	if !e.replacementConditionHolds(r.Params, source) {
		return false
	}
	return counterValidSA(e.G, t, r.Params["ValidSA"], o.Controller, source)
}

// counterValidSA is the Spell/Activated/Triggered subset used by R:Event$
// Counter. A qualifier scopes the stack object's controller relative to the
// replacement source; an unrecognised qualifier fails closed.
func counterValidSA(g *state.Game, target *state.Object, spec string, you state.PlayerID, source state.ObjID) bool {
	if spec == "" {
		return true
	}
	for _, alt := range strings.Split(spec, ",") {
		kind, quals, _ := strings.Cut(strings.TrimSpace(alt), ".")
		isKind := (kind == "Spell" && target.Ability == nil) ||
			(kind == "SpellAbility") ||
			(kind == "Activated" && target.Ability != nil && !isTriggered(g, target)) ||
			(kind == "Triggered" && target.Ability != nil && isTriggered(g, target))
		if !isKind {
			continue
		}
		if quals == "" {
			return true
		}
		// Spell qualifiers are card characteristics plus controller-relative
		// predicates. Reuse the ordinary object-filter grammar rather than a
		// hand-maintained qualifier allowlist, so Creature/Instant/colour/P/T
		// and future recognised predicates cannot drift from targeting.
		if target.Ability == nil && counterSpellQualifiers(g, target, quals, you, source) {
			return true
		}
		// Ability objects have no card face; their corpus qualifiers are the
		// controller-relative forms, evaluated explicitly against the wrapper.
		if target.Ability != nil {
			switch quals {
			case "YouCtrl":
				if target.Controller == you {
					return true
				}
			case "OppCtrl", "YouDontCtrl":
				if target.Controller != you {
					return true
				}
			}
		}
	}
	return false
}

func counterSpellQualifiers(g *state.Game, target *state.Object, quals string, you state.PlayerID, source state.ObjID) bool {
	var ordinary []string
	for _, q := range strings.Split(quals, "+") {
		switch q {
		case "hasKeywordFlash":
			if target.Face() == nil || !target.Face().HasKeyword("Flash") {
				return false
			}
		case "wasCastByYou":
			if target.Controller != you {
				return false
			}
		default:
			ordinary = append(ordinary, q)
		}
	}
	if len(ordinary) == 0 {
		return true
	}
	return effects.MatchesSpecFrom(g, "Card."+strings.Join(ordinary, "+"), target.ID, you, source)
}

func isTriggered(g *state.Game, o *state.Object) bool {
	_, ok := state.TriggerOf(g, o)
	return ok
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
	ev       events.Event
	cands    []replMatch
	used     []replMatch
	before   *triggerSnapshot // immutable SBA look-back, safe to share in Clone
	player   state.PlayerID
	damaging state.ObjID
	combat   bool
	lifelink bool
	deadly   bool
	counter  bool
	cause    state.ObjID
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
	e.replChoices = append(e.replChoices, replChoice{ev: ev, cands: matches, before: e.triggerBefore, player: p})
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
func (e *Engine) poseDamageReplacementChoice(ev events.Event, matches []replMatch, p state.PlayerID) {
	source := e.protectionSource(e.damaging)
	e.replChoices = append(e.replChoices, replChoice{
		ev: ev, cands: matches, before: e.triggerBefore, player: p,
		damaging: source, combat: e.combatDamaging,
		lifelink: e.HasKeyword(source, "Lifelink"), deadly: e.HasKeyword(source, "Deathtouch"),
	})
	if e.pending == nil {
		e.askReplacementChoice(p)
	}
}

func (e *Engine) askReplacementChoice(p state.PlayerID) {
	rc := e.replChoices[0]
	name := "this event"
	prompt := "Several replacement effects would modify damage: choose which applies next."
	if rc.ev.Kind == events.MoveZone {
		name = "this object"
		if o := e.G.Obj(rc.ev.Obj); o != nil && o.Face() != nil && o.Face().Name != "" {
			name = o.Face().Name
		}
		prompt = "Several replacement effects would change how " + name + " moves: choose the order they apply."
	}
	if rc.counter {
		prompt = "Several replacement effects would modify this counter event: choose which applies."
	}
	d := &decision.Decision{Player: p, Kind: decision.KReplacement, Min: 1, Max: 1,
		Prompt: prompt, Source: rc.ev.Obj, ResumeKind: "replacement"}
	for i, c := range rc.cands {
		label := "a replacement"
		if so := e.G.Obj(c.id); so != nil && so.Face() != nil && so.Face().Name != "" {
			label = "Apply " + so.Face().Name + "'s replacement"
		} else if dsc := c.repl.Params["Description"]; dsc != "" {
			label = "Apply: " + dsc
		}
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "replacement", Obj: c.id, Label: label})
	}
	if rc.ev.Kind == events.Damage && hasOptionalReplacement(rc.cands) {
		d.Options = append(d.Options, decision.Option{Index: len(rc.cands), Kind: "skip_replacement",
			Label: "Do not apply an optional replacement"})
	}
	// A replacement-order choice can arise in the middle of an effect's Emit.
	// Enter through Host.Ask so effects.Resolve sees Suspended and records the
	// remaining SA chain. A recomputation ask already owns that resume point;
	// pose it directly without overwriting the original continuation.
	if e.resume == nil {
		e.Ask(d)
	} else {
		e.ask(d)
	}
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
	rp := e.resume
	chosen := d.Chosen(in)
	if len(chosen) == 0 || chosen[0].Index < 0 || chosen[0].Index > len(rc.cands) ||
		(chosen[0].Index == len(rc.cands) && !(rc.ev.Kind == events.Damage && hasOptionalReplacement(rc.cands))) {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement-order answer out of range"})
		return
	}
	before := e.triggerBefore
	e.triggerBefore = rc.before
	completed := true
	switch {
	case rc.counter:
		e.applyChosenCounterReplacement(rc, chosen[0].Index)
	case rc.ev.Kind == events.Damage:
		completed = e.handleDamageReplacementChoice(rc, chosen[0].Index)
	default:
		e.applyReplacement(rc.ev, rc.cands[chosen[0].Index])
	}
	e.triggerBefore = before
	if completed && len(e.replChoices) > 0 {
		// The same parked resolution produced more than one replacement
		// choice: a multi-recipient DealDamage/DamageAll parks one per
		// recipient event before its enclosing chain suspends. The original
		// resume point must stay parked until the LAST of them is answered --
		// resuming after the first would run the remaining SA chain (and move
		// the spell off the stack) while a later recipient's damage is still
		// awaiting its CR 616.1 order choice, and the chained riders would
		// fire before the effect's own damage settled. If the application
		// itself posed a nested ask (e.resume no longer rp), chain this
		// frame's continuation behind the new one so nothing is dropped.
		if rp != nil && e.resume != rp {
			e.resume.outer = &resumePoint{kind: rp.kind, obj: rp.obj, outer: rp.outer}
		}
		for len(e.replChoices) > 0 {
			next := e.replChoices[0]
			// CR 616.1e: the affected player is recomputed from the parked
			// event at ask time, so a competition whose recipient changed
			// while parked asks the NEW affected player.
			if p, ok := e.damageAffectedPlayer(next.ev); ok && !e.G.Players[p].Lost {
				e.replChoices[0].player = p
				if e.pending == nil {
					e.askReplacementChoice(p)
				}
				return
			}
			// CR 800.4a: an affected player who has left the game or lost
			// makes no choices. Its candidates apply in deterministic scan
			// order and the queue drains on.
			e.replChoices = e.replChoices[1:]
			switch {
			case next.counter:
				e.applyChosenCounterReplacement(next, 0)
			case next.ev.Kind == events.Damage:
				completed = e.handleDamageReplacementChoice(next, 0)
			default:
				e.applyReplacement(next.ev, next.cands[0])
			}
			if !completed {
				return
			}
		}
	}
	if completed && rc.combat && e.combatRound.assignments != nil {
		// A combat damage pass was parked at this assignment. Finish the
		// remaining precomputed assignments before running SBAs or the regular
		// pass; a newly parked replacement simply returns again.
		e.damageStep(false)
		if e.pending == nil && e.combatRound.assignments == nil && e.combatRound.active {
			e.completeCombatPass(e.combatRound.pass)
		}
	}
	if completed && rp != nil && e.resume == rp {
		// The parked event and its riders are complete. Resume only the chain
		// after the effect that proposed it; the effect itself must not emit the
		// same damage/counter event a second time.
		e.resume = nil
		e.resumeResolution(rp, nil)
	}
}

// handleDamageReplacementChoice returns false only when recomputation leaves
// another genuine order choice pending; true means the parked damage event is
// fully prevented/replaced or has landed with all riders.
func (e *Engine) handleDamageReplacementChoice(rc replChoice, selected int) bool {
	savedDamaging, savedCombat := e.damaging, e.combatDamaging
	e.damaging, e.combatDamaging = rc.damaging, rc.combat
	defer func() { e.damaging, e.combatDamaging = savedDamaging, savedCombat }()
	var m replMatch
	if selected == len(rc.cands) {
		// This is the explicit "do not apply" answer for an Optional$ True
		// replacement. Mark every currently applicable optional replacement as
		// used so recomputation cannot immediately pose the same question again;
		// non-optional replacements remain eligible and still apply.
		for _, m := range rc.cands {
			if strings.EqualFold(m.repl.Params["Optional"], "True") {
				rc.used = append(rc.used, m)
			}
		}
	} else {
		m := rc.cands[selected]
		rc.used = append(rc.used, m)
		if e.applyChosenDamageReplacement(&rc.ev, m) {
			return true
		}
	}
	for {
		rc.cands = e.remainingDamageReplacements(rc.ev, rc.used)
		switch len(rc.cands) {
		case 0:
			e.finishChosenDamage(rc)
			return true
		case 1:
			m = rc.cands[0]
			rc.used = append(rc.used, m)
			if e.applyChosenDamageReplacement(&rc.ev, m) {
				return true
			}
		default:
			// The first modification can leave several effects applicable. Ask
			// again over exactly that recomputed set (CR 616.1e), preserving
			// the already-modified amount and the original damage rider
			// metadata. CR 616.1e also recomputes the AFFECTED player: after a
			// redirection the choice belongs to the new recipient's controller,
			// never the original one, so re-derive it from the modified event.
			if p, ok := e.damageAffectedPlayer(rc.ev); ok && !e.G.Players[p].Lost {
				rc.player = p
				e.replChoices = append([]replChoice{rc}, e.replChoices...)
				if e.pending == nil {
					e.askReplacementChoice(p)
				}
				return false
			}
			// CR 800.4a: the recomputed affected player is lost or gone and
			// makes no choices, so the remaining candidates apply in
			// deterministic scan order -- the same fallback the initial pose
			// takes -- and the parked event settles here.
			for {
				if len(rc.cands) == 0 {
					e.finishChosenDamage(rc)
					return true
				}
				m = rc.cands[0]
				rc.used = append(rc.used, m)
				if e.applyChosenDamageReplacement(&rc.ev, m) {
					return true
				}
				rc.cands = e.remainingDamageReplacements(rc.ev, rc.used)
			}
		}
	}
}

func (e *Engine) remainingDamageReplacements(ev events.Event, used []replMatch) []replMatch {
	var out []replMatch
	alreadyUsed := func(m replMatch) bool {
		for _, u := range used {
			if u.id == m.id && (u.repl == m.repl || (m.key != "" && u.key == m.key)) {
				return true
			}
		}
		return false
	}
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "" || ce.ReplacementBody == "" {
			continue
		}
		if with := replacementBodySA(ce.ReplacementBody); with != nil {
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: with}
			m := replMatch{id: ce.Source, repl: r,
				key: "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))}
			if !alreadyUsed(m) && e.replacementMatches(*r, ce.Source, ev) &&
				!(r.Params["Prevent"] == "True" && e.cantPreventDamage(e.damaging, ev.Obj)) {
				out = append(out, m)
			}
		}
	}
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			return
		}
		for i := range o.Face().Repls {
			r := &o.Face().Repls[i]
			if alreadyUsed(replMatch{id: id, repl: r}) || !e.replacementMatches(*r, id, ev) {
				continue
			}
			if r.Params["Prevent"] == "True" && e.cantPreventDamage(e.damaging, ev.Obj) {
				continue
			}
			out = append(out, replMatch{id: id, repl: r})
		}
	})
	return out
}

// applyChosenDamageReplacement modifies ev in place. It reports terminal when
// the chosen effect prevented/replaced the damage entirely; ReplaceEffect is
// nonterminal and lets applicability be recomputed against its new amount.
func (e *Engine) applyChosenDamageReplacement(ev *events.Event, m replMatch) bool {
	if m.repl.Params["Prevent"] == "True" {
		return true
	}
	if m.repl.With == nil {
		return true
	}
	e.runReplaceWith(e.replCtx(m, *ev), ev.Obj, m.repl.With, ev)
	return m.repl.With.API != "ReplaceEffect"
}

func (e *Engine) finishChosenDamage(rc replChoice) {
	savedDamaging, savedCombat, savedApplying := e.damaging, e.combatDamaging, e.applyingReplacement
	e.damaging, e.combatDamaging, e.applyingReplacement = rc.damaging, rc.combat, true
	applied := e.emit(rc.ev)
	e.damaging, e.combatDamaging, e.applyingReplacement = savedDamaging, savedCombat, savedApplying
	if applied.Kind != events.Damage || applied.Amount <= 0 {
		return
	}
	if rc.deadly && applied.Obj != 0 {
		e.emit(events.Event{Kind: events.CounterChange, Obj: applied.Obj,
			Counter: "Deathtouched", Amount: 1})
	}
	if rc.lifelink {
		e.emit(events.Event{Kind: events.LifeChange, Player: e.controllerOf(rc.damaging), Amount: applied.Amount})
	}
	if rc.combat && applied.Obj == 0 && e.format == FormatCommander {
		e.tallyCmdDamage(applied.Player, rc.damaging, applied.Amount)
	}
}

func init() {
	// kw:etbCounter and kw:ETBReplacement are implemented wholly by the
	// machinery above: both are R:Event$ Moved replacements (expanded from a
	// K: line by cards/keywords.go) matched and applied here. Reading a card's
	// own tags is what a replacement registration means -- nothing elsewhere
	// in the tree registers them.
	effects.RegisterNonAPI("kw:etbCounter", "kw:ETBReplacement", "repl:DamageDone", "repl:Counter")
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
	ev     events.Event
	obj    state.ObjID
	before *triggerSnapshot // immutable SBA look-back, safe to share in Clone
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
	e.cmdZone = append(e.cmdZone, cmdZoneMove{ev: ev, obj: ev.Obj, before: e.triggerBefore})
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
	saved, before := e.applyingReplacement, e.triggerBefore
	e.applyingReplacement, e.triggerBefore = true, pm.before
	e.emit(events.Event{Kind: events.MoveZone, Obj: pm.ev.Obj, From: pm.ev.From,
		To: to, Player: pm.ev.Player, Text: pm.ev.Text})
	e.applyingReplacement, e.triggerBefore = saved, before
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
