// replacement.go applies R: line replacement effects ahead of an event's
// own logging, from engine.go's emit. applyReplacements discovers every
// applicable effect in deterministic order, then applies the event-specific
// CR 616 ordering and optional-effect rules.
package rules

import (
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
// Matches resolve ReplaceWith$ inside applyingReplacement, so nested emits
// cannot recursively apply the same replacement forever. MoveZone,
// BeginPhase and ProduceMana competitions park the event for the affected
// player's CR 616 order choice; the event-specific handlers below describe
// when the chosen effect completes the event and when remaining effects are
// reconsidered.
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
	// Positive LifeChange is a gain; negative LifeChange and player Damage
	// are life loss. Both route to the life replacement machinery below,
	// ahead of the Moved/Untap/BeginPhase/Transform/ProduceMana mapping,
	// which does not cover life events.
	if ev.Kind == events.LifeChange || (ev.Kind == events.Damage && ev.Obj == 0) {
		return e.applyLifeReplacements(ev)
	}
	event, ok := replacementEvent(ev)
	if !ok {
		return ev, false
	}
	// Madness is an optional discard replacement and must park before either
	// destination is logged. The guarded re-emit still permits ordinary card
	// and format replacements on the chosen destination.
	if ev.Kind == events.MoveZone && !e.applyingMadnessChoice && e.madnessReplacementApplies(ev) {
		e.parkMadnessDiscard(ev)
		return ev, true
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
	if ev.Kind == events.MoveZone && e.commanderZoneReplacementApplies(ev) {
		e.parkCommanderZoneMove(ev)
		return ev, true
	}
	// Collect EVERY replacement effect this event matches, in
	// forEachObject's deterministic scan order, rather than the single first
	// match the M1 build took.
	var matches, manaCandidates []replMatch
	e.forEachReplacementSource(func(id state.ObjID) {
		for _, f := range e.replacementFaces(id, ev) {
			for i := range f.Repls {
				if f.Repls[i].Event != event {
					continue
				}
				m := replMatch{id: id, face: f, repl: &f.Repls[i]}
				// Mana replacement applicability must be re-evaluated after
				// every rewrite (CR 616.1). Keep even the candidates that do
				// not match the initial amount: multiplying mana can make a
				// later ManaAmount$ gate newly applicable.
				if ev.Kind == events.ManaAdd {
					manaCandidates = append(manaCandidates, m)
				}
				if e.replacementMatches(f.Repls[i], id, ev) {
					matches = append(matches, m)
				}
			}
		}
	})
	if ev.Kind == events.ManaAdd {
		return e.continueManaReplacements(ev, manaCandidates, nil, false, e.manaFromTap, e.manaProducer)
	}
	if len(matches) == 0 {
		return ev, false
	}
	switch ev.Kind {
	case events.Untap:
		return e.continueUntapReplacements(ev, matches)
	case events.StepChange:
		return e.continuePhaseReplacements(ev, matches, nil)
	case events.FlipFace:
		return e.applyTransformReplacement(ev, matches)
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

// replMatch is one replacement effect the engine found applicable to an
// event: its owning source permanent and the R: line on that permanent's
// face. A plain value, cloned by copy.
type replMatch struct {
	id   state.ObjID
	face *cards.Face // prospective face for an "as this transforms" replacement
	repl *cards.Repl
}

// forEachReplacementSource extends the ordinary battlefield/game-zone scan
// with the command zone, where Plane/Vanguard replacement text explicitly
// declares ActiveZones$ Command. Trigger discovery deliberately keeps using
// forEachObject, so this cannot make unrelated command-zone triggers live.
// Command-zone objects are visited once per living seat, after all ordinary
// zones, in their zone order; replacementMatches requires an explicit Command
// ActiveZones declaration there, preventing ordinary card text from becoming
// active merely because its object happens to be parked in that zone.
func (e *Engine) forEachReplacementSource(fn func(id state.ObjID)) {
	e.forEachObject(fn)
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZCommand, p) {
			fn(id)
		}
	}
}

// replacementEvent maps the event log's concrete events to Forge R:Event$
// names. ManaAdd's producer and tap provenance live in synchronous Engine
// scratch instead of its hash-chained fields; ProduceMana matching requires
// both, while the logged event remains the ordinary final mana production.
func replacementEvent(ev events.Event) (string, bool) {
	switch ev.Kind {
	case events.MoveZone:
		return "Moved", true
	case events.Untap:
		return "Untap", true
	case events.StepChange:
		return "BeginPhase", true
	case events.FlipFace:
		return "Transform", true
	case events.ManaAdd:
		return "ProduceMana", true
	default:
		return "", false
	}
}

// replacementFaces returns the source faces whose R: lines apply now. A
// transform's "as this transforms into ..." replacement belongs to the
// destination face, while every other replacement reads the source's current
// face. This avoids making the alternate face live for unrelated events.
func (e *Engine) replacementFaces(id state.ObjID, ev events.Event) []*cards.Face {
	o := e.G.Obj(id)
	if o == nil || o.Card == nil {
		return nil
	}
	if ev.Kind == events.FlipFace && id == ev.Obj && ev.Amount >= 0 && int(ev.Amount) < len(o.Card.Faces) {
		return []*cards.Face{o.Card.Faces[ev.Amount]}
	}
	if f := o.Face(); f != nil {
		return []*cards.Face{f}
	}
	return nil
}

// continueUntapReplacements applies the sole replacement automatically, but
// parks a competition for the untapped permanent's controller. Each Untap
// replacement prevents the original event (and may run its ReplaceWith$), so
// the chosen effect completes the event and the others get no second pass.
// This is the same CR 616.1 affected-player choice as MoveZone, with Untap's
// event-specific applySimpleReplacement semantics.
func (e *Engine) continueUntapReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	if len(matches) == 1 {
		return e.applySimpleReplacement(ev, matches[0])
	}
	o := e.G.Obj(ev.Obj)
	if o != nil && int(o.Controller) < len(e.G.Players) && !e.G.Players[o.Controller].Lost {
		e.poseUntapReplacementChoice(ev, matches)
		return ev, true
	}
	// A departed affected player cannot choose; retain the deterministic scan
	// order fallback used by the other replacement competitions.
	return e.applySimpleReplacement(ev, matches[0])
}

// applySimpleReplacement handles events whose replacement prevents the event
// (no ReplaceWith$, the CantHappen form) or replaces it with a body. Untap is
// the important example: an ordinary activated DB$ Untap is outside the
// untap step and consequently does not match ValidStepTurnToController$.
func (e *Engine) applySimpleReplacement(ev events.Event, m replMatch) (events.Event, bool) {
	if m.repl.With != nil {
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, "", m.repl.With)
	}
	return ev, true
}

// applyBeginPhaseReplacement skips the phase by emitting the following phase
// entry. The skipped StepChange never enters the log, so replay performs
// exactly the same transition without needing an ephemeral "skipped" bit in
// game state. The skip itself is emitted through the UNGUARDED emit --
// applyingReplacement is false here, outside runReplaceWith -- so a chain of
// skips (one effect skipping untap, another upkeep) keeps skipping: the step
// number strictly increases, so the recursion terminates. The landing step's
// own StepChange is the log entry that exists, and every Phase trigger sees
// exactly the steps that actually happened.
func (e *Engine) applyBeginPhaseReplacement(ev events.Event, m replMatch) (events.Event, bool) {
	if m.repl.With != nil {
		e.runReplaceWith(e.replCtx(m, ev), 0, "", m.repl.With)
	}
	// Cleanup is never skipped: the turn's 514.1/514.2 work is what makes the
	// next turn begin correctly, and no corpus line names it. Bounding the
	// emission here also bounds the chain recursion.
	if ev.Step < state.StepCleanup {
		e.emit(events.Event{Kind: events.StepChange, Step: ev.Step + 1})
	}
	return ev, true
}

// continuePhaseReplacements processes every replacement applicable to one
// proposed step entry. With several effects the active (affected) player
// chooses which gets the first opportunity under CR 616.1. Choosing an
// optional effect leads to its separate apply/decline ask; declining marks
// only that effect used and continues through the remaining mandatory or
// optional effects instead of bypassing replacement matching on the parked
// StepChange. Applying any effect skips the step and completes this event.
func (e *Engine) continuePhaseReplacements(ev events.Event, candidates []replMatch, used []bool) (events.Event, bool) {
	if used == nil {
		used = make([]bool, len(candidates))
	}
	applicable := e.applicablePhaseReplacements(ev, candidates, used)
	if len(applicable) == 0 {
		return ev, false
	}
	if len(applicable) > 1 && int(e.G.Active) < len(e.G.Players) && !e.G.Players[e.G.Active].Lost {
		e.posePhaseOrderChoice(ev, candidates, used, applicable)
		return ev, true
	}
	i := applicable[0]
	if candidates[i].repl.Params["Optional"] == "True" && int(e.G.Active) < len(e.G.Players) &&
		!e.G.Players[e.G.Active].Lost {
		e.posePhaseOptionalChoice(ev, candidates, used, i)
		return ev, true
	}
	return e.applyBeginPhaseReplacement(ev, candidates[i])
}

func (e *Engine) applicablePhaseReplacements(ev events.Event, candidates []replMatch, used []bool) []int {
	var applicable []int
	for i, m := range candidates {
		if !used[i] && e.replacementMatches(*m.repl, m.id, ev) {
			applicable = append(applicable, i)
		}
	}
	return applicable
}

// finishParkedPhase settles the old step boundary exactly once, then either
// applies the selected skip or enters the original proposed step under a
// guard (all applicable replacements have already had their opportunity).
func (e *Engine) finishParkedPhase(rc replChoice, selected int) {
	if rc.boundary {
		e.finishStepBoundary(rc.leaving, rc.ev.Step)
	}
	if selected >= 0 {
		e.applyBeginPhaseReplacement(rc.ev, rc.cands[selected])
	} else {
		saved := e.applyingReplacement
		e.applyingReplacement = true
		e.emit(rc.ev)
		e.applyingReplacement = saved
	}
	if e.pending == nil {
		e.finishEnteredStep()
	}
}

// resumeParkedPhase continues after an optional replacement was declined.
// It preserves the original boundary ownership while either posing the next
// order/optional ask or completing with the remaining mandatory replacement.
func (e *Engine) resumeParkedPhase(rc replChoice) {
	applicable := e.applicablePhaseReplacements(rc.ev, rc.cands, rc.applied)
	if len(applicable) == 0 {
		e.finishParkedPhase(rc, -1)
		return
	}
	if len(applicable) > 1 && int(e.G.Active) < len(e.G.Players) && !e.G.Players[e.G.Active].Lost {
		rc.kind = replChoicePhaseOrder
		rc.applicable = append(rc.applicable[:0], applicable...)
		e.replChoices = append([]replChoice{rc}, e.replChoices...)
		return
	}
	i := applicable[0]
	if rc.cands[i].repl.Params["Optional"] == "True" && int(e.G.Active) < len(e.G.Players) &&
		!e.G.Players[e.G.Active].Lost {
		rc.kind = replChoicePhaseOptional
		rc.selected = i
		rc.applicable = nil
		e.replChoices = append([]replChoice{rc}, e.replChoices...)
		return
	}
	e.finishParkedPhase(rc, i)
}

// applyTransformReplacement lets every matching "as this transforms" body
// resolve, then leaves the FlipFace event intact. Forge writes these as an
// augmentation (Sephiroth gains its emblem as it becomes the Angel), not as
// a substitute that cancels the transformation.
func (e *Engine) applyTransformReplacement(ev events.Event, matches []replMatch) (events.Event, bool) {
	for _, m := range matches {
		if m.repl.With != nil {
			e.runReplaceWith(e.replCtx(m, ev), ev.Obj, "", m.repl.With)
		}
	}
	return ev, false
}

// continueManaReplacements implements CR 616.1 for one in-flight ManaAdd.
// Each replacement may apply once. After every rewrite the full candidate set
// is re-checked against the NEW amount/type; if several apply, the player
// receiving the mana chooses which is applied next. A lone applicable effect
// is automatic. Only the final rewritten ManaAdd enters the event log, so a
// log-only replay needs no transient provenance or replacement state.
func (e *Engine) continueManaReplacements(ev events.Event, candidates []replMatch,
	applied []bool, changed, tapped bool, producer state.ObjID) (events.Event, bool) {
	if applied == nil {
		applied = make([]bool, len(candidates))
	}
	// A colour/order answer resumes after resolveManaEffectColor restored its
	// synchronous scratch. Rebind producer for every applicability recheck so
	// a parked replacement still sees the permanent that produced this mana.
	savedProducer := e.manaProducer
	e.manaProducer = producer
	defer func() { e.manaProducer = savedProducer }()
	for {
		var applicable []int
		savedTap := e.manaFromTap
		e.manaFromTap = tapped
		for i, m := range candidates {
			if !applied[i] && e.replacementMatches(*m.repl, m.id, ev) {
				applicable = append(applicable, i)
			}
		}
		e.manaFromTap = savedTap
		if len(applicable) == 0 {
			if !changed {
				return ev, false
			}
			stored := events.Emit(e.G, e.L, ev)
			e.checkTriggers(stored, nil, 0, 0, false)
			return stored, true
		}
		if len(applicable) > 1 && int(ev.Player) < len(e.G.Players) && !e.G.Players[ev.Player].Lost {
			e.poseManaReplacementChoice(ev, candidates, applied, applicable, changed, tapped, producer)
			return ev, true
		}
		// A sole applicable replacement is mandatory. A choice-valued colour
		// still belongs to the affected player, so park the in-flight event
		// before applying it. A departed player cannot answer and deterministically
		// takes the first colour, just as it takes the first competing effect.
		i := applicable[0]
		if manaReplacementNeedsColor(candidates[i]) && int(ev.Player) < len(e.G.Players) &&
			!e.G.Players[ev.Player].Lost {
			e.poseManaColorReplacementChoice(ev, candidates, applied, i, changed, tapped, producer)
			return ev, true
		}
		choice := ""
		if manaReplacementNeedsColor(candidates[i]) {
			choice = "W"
		}
		ev = e.applyOneManaReplacement(ev, candidates[i], choice)
		applied[i] = true
		changed = true
	}
}

// applyOneManaReplacement applies a body while its producer is bound in
// Engine scratch by continueManaReplacements (or by the resume wrapper).
func (e *Engine) applyOneManaReplacement(ev events.Event, m replMatch, color string) events.Event {
	if m.repl.With == nil {
		return ev
	}
	ctx := &effects.Ctx{Source: m.id, Controller: e.controllerOf(m.id),
		ManaAmount: ev.Amount, ManaType: ev.Counter, ManaChoice: color}
	if m.face != nil {
		effects.SetSVars(ctx, m.face.SVars)
	}
	// The producer is contextual (not ManaAdd.Obj), but ReplaceWith$ still
	// resolves against that object for Defined$/Remembered$ references.
	e.runReplaceWith(ctx, e.manaProducer, "", m.repl.With)
	ev.Amount, ev.Counter = ctx.ManaAmount, ctx.ManaType
	return ev
}

// applyOneManaReplacementWithProducer restores the contextual producer for
// the one rewrite that occurs immediately after an answered replacement
// decision; continueManaReplacements then rebinds it for later rechecks.
func (e *Engine) applyOneManaReplacementWithProducer(ev events.Event, m replMatch, color string, producer state.ObjID) events.Event {
	saved := e.manaProducer
	e.manaProducer = producer
	defer func() { e.manaProducer = saved }()
	return e.applyOneManaReplacement(ev, m, color)
}

// manaReplacementNeedsColor identifies every choice-valued spelling the
// ReplaceMana primitive accepts. It follows effReplaceMana's precedence
// (ReplaceMana, then ReplaceType, then ReplaceColor), so an ignored lower-
// precedence parameter cannot accidentally pose a second choice.
func manaReplacementNeedsColor(m replMatch) bool {
	if m.repl == nil || m.repl.With == nil {
		return false
	}
	p := m.repl.With.Params
	kind := strings.TrimSpace(p["ReplaceMana"])
	if kind == "" {
		kind = strings.TrimSpace(p["ReplaceType"])
	}
	if kind == "" {
		kind = strings.TrimSpace(p["ReplaceColor"])
	}
	return strings.EqualFold(kind, "Any") || strings.EqualFold(kind, "Chosen")
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
		Captured:   []state.Target{{Obj: ev.Obj}},
		// Replaced names the object the replaced event (ev) was about, so a
		// ReplaceWith$ that says Defined$ ReplacedCard (the Rest in Peace /
		// Dryad Militant / Leyline of the Void shape: "exile it instead") can
		// act on exactly the card being kept out of the graveyard -- not the
		// source that owns the replacement.
		Replaced: ev.Obj}
	f := m.face
	if f == nil {
		f = o.Face()
	}
	if f != nil {
		effects.SetSVars(ctx, f.SVars)
	}
	return ctx
}

// runReplaceWith resolves one ReplaceWith$ body inside the replacement
// re-entrancy guard (CR 616.1, a replacement applies once), recording the
// replaced object for the body's Defined$/Remembered$ reads and restoring
// both the guard and that record afterward so an outer replacement keeps its
// own state.
func (e *Engine) runReplaceWith(ctx *effects.Ctx, replaced state.ObjID, action string, with *cards.SA) {
	savedRepl, savedAction := e.replReplaced, e.replAction
	e.applyingReplacement = true
	e.replReplaced, e.replAction = replaced, action
	e.resolveReplacementWith(ctx, with)
	e.replReplaced, e.replAction = savedRepl, savedAction
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
		departing, link, controller := e.captureSourceLifelinkLKI(ev)
		stored := events.Emit(e.G, e.L, ev)
		e.checkTriggers(stored, nil, 0, 0, false)
		e.finishSourceLifelinkLKI(ev, departing, link, controller)
		e.runReplaceWith(ctx, ev.Obj, "", m.repl.With)
		return stored, true
	}
	e.runReplaceWith(ctx, ev.Obj, events.ActionMarker(ev), m.repl.With)
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
	departing, link, controller := e.captureSourceLifelinkLKI(ev)
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored, nil, 0, 0, false)
	e.finishSourceLifelinkLKI(ev, departing, link, controller)
	for _, m := range matches {
		if m.repl.With == nil {
			continue
		}
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, "", m.repl.With)
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

// replacementMatches implements the per-event match predicates for the five
// replacement events the engine routes through applyReplacements: R:Event$
// Moved (Origin$/Destination$/ValidCard$/ValidLKI$), Untap (the "doesn't
// untap during its controller's untap step" class), BeginPhase (the
// "skip your draw step" class), Transform (the "as this transforms" class)
// and ProduceMana (the "produces three times as much" class). All five share
// the ActiveZones$ gate; the per-event parameters each fail closed on a
// value this build cannot evaluate, the same contract filter.go's matcher
// gives card filters.
func (e *Engine) replacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	// The generic object walk historically excludes the command zone. Its
	// replacement-only extension admits command-zone sources, but only when
	// the script explicitly declares that zone; this keeps ordinary card text
	// parked there inert and does not change trigger discovery.
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZCommand {
		active, ok := r.Params["ActiveZones"]
		if !ok || !zoneSpecContains(active, state.ZCommand) {
			return false
		}
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
	// on the stack. ev.To is only meaningful for a MoveZone; the other four
	// events leave it at its zero value, so the prospective clause is
	// MoveZone-only rather than reading a meaningless zero zone.
	if active, ok := r.Params["ActiveZones"]; ok {
		o := e.G.Obj(source)
		currentlyActive := o != nil && zoneSpecContains(active, o.Zone)
		enteringActive := ev.Kind == events.MoveZone && source == ev.Obj &&
			zoneSpecContains(active, ev.To)
		if !currentlyActive && !enteringActive {
			return false
		}
	}
	you := e.controllerOf(source)
	switch r.Event {
	case "Moved":
		if ev.Kind != events.MoveZone {
			return false
		}
		// Discard$ True narrows a Moved replacement to a discard. EffectOnly$
		// excludes cost and cleanup discards; ValidCause$ names its cause.
		if r.Params["Discard"] == "True" {
			if !events.IsDiscard(ev) {
				return false
			}
			if r.Params["EffectOnly"] == "True" && (events.IsDiscardCost(ev) || e.actionCause() == 0) {
				return false
			}
			if spec := r.Params["ValidCause"]; spec != "" && !e.discardCauseAdmits(spec, source, ev) {
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
			if !effects.MatchesSpecFrom(e.G, v, ev.Obj, you, source) {
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
				You: you, Source: source}) {
				return false
			}
		}
		return true
	case "Untap":
		if ev.Kind != events.Untap {
			return false
		}
		// ValidStepTurnToController$ You scopes the replacement to the untap
		// step's own turn-based action -- an activated or triggered untap
		// outside that step is not replaced (Basalt Monolith can still pay {3}
		// to untap itself). "Its controller's untap step" reads against the
		// card being untapped, which is what every corpus description says
		// (Sleep Paralysis's enchanted artifact vs. Basalt's itself); for a
		// ValidCard$ Card.Self line the two are the same player. A value other
		// than You fails closed.
		if s, ok := r.Params["ValidStepTurnToController"]; ok {
			if s != "You" || e.G.Step != state.StepUntap ||
				e.G.Active != e.controllerOf(ev.Obj) {
				return false
			}
		}
		if v, ok := r.Params["ValidCard"]; ok &&
			!effects.MatchesSpecFrom(e.G, v, ev.Obj, you, source) {
			return false
		}
		return e.replacementConditionHolds(r, source, you)
	case "BeginPhase":
		if ev.Kind != events.StepChange {
			return false
		}
		if ph, ok := r.Params["Phase"]; ok {
			step, known := phaseStep(ph)
			if !known || step != ev.Step {
				return false
			}
		}
		// ValidPlayer$ You scopes "skip YOUR draw step" to the replacement
		// controller's own turn; a line with no ValidPlayer$ (Sands of Time's
		// "players skip their untap step") applies every turn.
		if vp, ok := r.Params["ValidPlayer"]; ok &&
			!effects.MatchesPlayerSpec(e.G, vp, e.G.Active, you) {
			return false
		}
		// Optional$ True is handled after matching by
		// posePhaseReplacementChoice: applicability is independent of whether
		// the affected player eventually chooses to apply it.
		// Hellbent$ True gates the skip on an empty hand (one corpus line).
		if r.Params["Hellbent"] == "True" && len(e.G.Zone(state.ZHand, you)) > 0 {
			return false
		}
		return e.replacementConditionHolds(r, source, you)
	case "Transform":
		if ev.Kind != events.FlipFace {
			return false
		}
		// The "as this transforms" replacement is written on the destination
		// face and applies to its own card's flip; replacementFaces already
		// scanned the destination face for this event.
		if v, ok := r.Params["ValidCard"]; ok &&
			!effects.MatchesSpecFrom(e.G, v, ev.Obj, you, source) {
			return false
		}
		return e.replacementConditionHolds(r, source, you)
	case "ProduceMana":
		// Only genuine production replaces: a ManaAdd without a producing
		// source (a test seed, a spend) and a negative Amount (spending, not
		// producing) are outside the class.
		if ev.Kind != events.ManaAdd || e.manaProducer == 0 || ev.Amount <= 0 || !e.manaFromTap {
			return false
		}
		if v, ok := r.Params["ValidCard"]; ok &&
			!effects.MatchesSpecFrom(e.G, v, e.manaProducer, you, source) {
			return false
		}
		// ValidActivator$ You: the player adding the mana (whoever activated
		// the mana ability) must be the replacement controller's side of the
		// spec. MatchesPlayerSpec fails closed on unknown qualifiers.
		if va, ok := r.Params["ValidActivator"]; ok &&
			!effects.MatchesPlayerSpec(e.G, va, ev.Player, you) {
			return false
		}
		// ReplaceOnly$ lives on the ReplaceWith$ body (Quarum Trench
		// Gnomes), but it is an applicability gate: converting a different
		// colour is not applying that replacement. Reading it here lets a
		// prior rewrite make the effect newly applicable during CR 616.1's
		// mandatory post-rewrite recheck.
		if r.With != nil {
			if only := strings.TrimSpace(r.With.Params["ReplaceOnly"]); only != "" && only != ev.Counter {
				return false
			}
		}
		// ManaAmount$ <op><n> gates on the size of the production being
		// replaced (Damping Sphere's "two or more mana").
		if ma, ok := r.Params["ManaAmount"]; ok {
			op, n, parsed := splitCompare(ma)
			if !parsed || !applyCompare(int(ev.Amount), op, n) {
				return false
			}
		}
		return e.replacementConditionHolds(r, source, you)
	}
	return false
}

// phaseStep maps a Forge Phase$ value on a BeginPhase replacement onto the
// engine's step constants. Only the three steps the corpus names are known;
// any other value (or a comma list this build does not split) fails closed,
// leaving the phase to run normally.
func phaseStep(ph string) (state.Step, bool) {
	switch strings.TrimSpace(ph) {
	case "Untap":
		return state.StepUntap, true
	case "Upkeep":
		return state.StepUpkeep, true
	case "Draw":
		return state.StepDraw, true
	}
	return 0, false
}

// replacementConditionHolds evaluates the condition parameters a replacement
// R: line can carry besides its event gates: IsPresent$ with an optional
// PresentCompare$ (default "at least one", the intervening-if reading the
// corpus's aura lines use: "if you control a Reflection"), and
// CheckSVar$/SVarCompare$ (an SVar value compared against a threshold). The
// SVar is evaluated in the replacement source's own context, exactly as a
// trigger's condition would be. A clause this build cannot evaluate -- an
// unknown compare literal, a missing SVar, a non-Count$ body -- fails
// closed: the replacement does not apply, never that an unreadable count is
// presumed large enough to let it.
func (e *Engine) replacementConditionHolds(r cards.Repl, source state.ObjID, you state.PlayerID) bool {
	if spec, ok := r.Params["IsPresent"]; ok {
		cmp := r.Params["PresentCompare"]
		if cmp == "" {
			cmp = "GE1"
		}
		if !comparePresent(e.countPresent(spec, source, you), cmp) {
			return false
		}
	}
	if name, ok := r.Params["CheckSVar"]; ok {
		op, n, parsed := splitCompare(r.Params["SVarCompare"])
		if !parsed {
			return false
		}
		ctx := &effects.Ctx{Source: source, Controller: you}
		if o := e.G.Obj(source); o != nil && o.Face() != nil {
			effects.SetSVars(ctx, o.Face().SVars)
		}
		if !applyCompare(int(effects.EvalCount(e, ctx, ctx.SVars[name])), op, n) {
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
type replChoiceKind uint8

const (
	replChoiceMove replChoiceKind = iota
	replChoiceMana
	replChoiceManaColor
	replChoicePhaseOrder
	replChoicePhaseOptional
	// replChoiceUntap parks competing effects that would replace one Untap.
	// It is appended so existing in-memory enum values remain unchanged.
	replChoiceUntap
)

type replChoice struct {
	kind         replChoiceKind
	ev           events.Event
	cands        []replMatch
	applied      []bool      // mana/phase: candidates that already had their opportunity
	applicable   []int       // order decision option -> candidate index
	selected     int         // mana colour / phase optional: candidate awaiting its answer
	changed      bool        // mana: at least one rewrite already happened
	manaTapped   bool        // mana: tap provenance survives the decision boundary
	manaProducer state.ObjID // mana: producer survives the decision boundary
	boundary     bool        // phase: this choice owns setStep's boundary cleanup
	leaving      state.Step
	before       *triggerSnapshot // immutable SBA look-back, safe to share in Clone
	untap        *untapStep       // remaining turn-based untaps after an order answer
}

// poseReplacementChoice starts a CR 616.1 order-selection suspension: the
// competing event is parked and the affected controller -- the controller of
// the moving object, CR 616.1's "affected player" -- is asked which
// replacement applies first. Mirrors commander-zone parking: the ask is posed
// only when no other decision is already pending (the caller has already
// ruled out a departed controller, which makes no choices under CR 800.4a).
// poseUntapReplacementChoice starts the CR 616.1 choice between effects
// replacing one Untap. The affected player is the untapped object's
// controller, not either replacement source's controller.
func (e *Engine) poseUntapReplacementChoice(ev events.Event, matches []replMatch) {
	o := e.G.Obj(ev.Obj)
	if o == nil || int(o.Controller) >= len(e.G.Players) {
		return
	}
	rc := replChoice{kind: replChoiceUntap, ev: ev, cands: matches, before: e.triggerBefore}
	if e.untapResume != nil {
		resume := *e.untapResume
		rc.untap = &resume
	}
	e.replChoices = append(e.replChoices, rc)
	if e.pending == nil {
		e.askReplacementChoice(o.Controller)
	}
}

func (e *Engine) poseReplacementChoice(ev events.Event, matches []replMatch) {
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return
	}
	p := o.Controller
	if int(p) >= len(e.G.Players) {
		return
	}
	e.replChoices = append(e.replChoices, replChoice{kind: replChoiceMove,
		ev: ev, cands: matches, before: e.triggerBefore})
	if e.pending == nil {
		e.askReplacementChoice(p)
	}
}

// poseManaReplacementChoice parks a partially rewritten mana event until the
// player receiving it chooses the next applicable effect (CR 616.1). The full
// candidate set and applied bitmap survive the choice so applicability can be
// re-evaluated after the selected rewrite, including effects newly enabled by
// a changed ManaAmount$.
func (e *Engine) poseManaReplacementChoice(ev events.Event, candidates []replMatch,
	applied []bool, applicable []int, changed, tapped bool, producer state.ObjID) {
	e.replChoices = append(e.replChoices, replChoice{kind: replChoiceMana, ev: ev,
		cands: candidates, applied: append([]bool(nil), applied...),
		applicable: append([]int(nil), applicable...), changed: changed,
		manaTapped: tapped, manaProducer: producer, before: e.triggerBefore})
	if e.pending == nil {
		e.askReplacementChoice(ev.Player)
	}
}

// poseManaColorReplacementChoice parks a partially rewritten mana event while
// the receiving player chooses W/U/B/R/G for one choice-valued ReplaceMana
// body. Candidate state and the applied bitmap are retained so the answer can
// resume the same CR 616.1 applicability loop.
func (e *Engine) poseManaColorReplacementChoice(ev events.Event, candidates []replMatch,
	applied []bool, selected int, changed, tapped bool, producer state.ObjID) {
	e.replChoices = append(e.replChoices, replChoice{kind: replChoiceManaColor, ev: ev,
		cands: candidates, applied: append([]bool(nil), applied...), selected: selected,
		changed: changed, manaTapped: tapped, manaProducer: producer, before: e.triggerBefore})
	if e.pending == nil {
		e.askReplacementChoice(ev.Player)
	}
}

func (e *Engine) phaseChoice(ev events.Event, candidates []replMatch, used []bool) replChoice {
	rc := replChoice{ev: ev, cands: candidates, applied: append([]bool(nil), used...),
		before: e.triggerBefore}
	if e.stepLeaving != nil {
		rc.boundary = true
		rc.leaving = *e.stepLeaving
	}
	return rc
}

// posePhaseOrderChoice parks a step entry with all currently applicable
// BeginPhase replacements so the active player chooses which gets the first
// opportunity under CR 616.1.
func (e *Engine) posePhaseOrderChoice(ev events.Event, candidates []replMatch,
	used []bool, applicable []int) {
	rc := e.phaseChoice(ev, candidates, used)
	rc.kind = replChoicePhaseOrder
	rc.applicable = append([]int(nil), applicable...)
	e.replChoices = append(e.replChoices, rc)
	if e.pending == nil {
		e.askReplacementChoice(e.G.Active)
	}
}

// posePhaseOptionalChoice parks the selected Optional$ BeginPhase replacement
// for its apply/decline answer. Declining resumes the remaining candidate set.
func (e *Engine) posePhaseOptionalChoice(ev events.Event, candidates []replMatch,
	used []bool, selected int) {
	rc := e.phaseChoice(ev, candidates, used)
	rc.kind = replChoicePhaseOptional
	rc.selected = selected
	e.replChoices = append(e.replChoices, rc)
	if e.pending == nil {
		e.askReplacementChoice(e.G.Active)
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
	d := &decision.Decision{Player: p, Kind: decision.KReplacement, Min: 1, Max: 1,
		Source: rc.ev.Obj}
	indices := make([]int, len(rc.cands))
	for i := range indices {
		indices[i] = i
	}
	switch rc.kind {
	case replChoiceMana:
		d.Prompt = "Several replacement effects would change mana production: choose which applies next."
		indices = rc.applicable
	case replChoiceManaColor:
		d.Prompt = "Choose the colour of the replacement mana."
		for i, color := range []string{"W", "U", "B", "R", "G"} {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: "mana", Obj: rc.cands[rc.selected].id,
				Label: "Add " + color})
		}
		e.ask(d)
		return
	case replChoicePhaseOrder:
		d.Prompt = "Several replacement effects would change this step: choose which applies first."
		indices = rc.applicable
	case replChoicePhaseOptional:
		m := rc.cands[rc.selected]
		name := "this replacement effect"
		if so := e.G.Obj(m.id); so != nil && so.Face() != nil && so.Face().Name != "" {
			name = so.Face().Name
		}
		d.Prompt = "Apply " + name + "'s optional replacement and skip this step?"
		d.Options = []decision.Option{
			{Index: 0, Kind: "apply", Obj: m.id, Label: "Yes — skip this step"},
			{Index: 1, Kind: "decline", Obj: m.id, Label: "No — do not apply this replacement"},
		}
		e.ask(d)
		return
	case replChoiceUntap:
		name := "this object"
		if o := e.G.Obj(rc.ev.Obj); o != nil && o.Face() != nil && o.Face().Name != "" {
			name = o.Face().Name
		}
		d.Prompt = "Several replacement effects would change how " + name + " untaps: choose which applies first."
	default:
		name := "this object"
		if o := e.G.Obj(rc.ev.Obj); o != nil && o.Face() != nil && o.Face().Name != "" {
			name = o.Face().Name
		}
		d.Prompt = "Several replacement effects would change how " + name + " moves: choose which applies first."
	}
	for _, candidate := range indices {
		c := rc.cands[candidate]
		label := "Apply a replacement"
		if so := e.G.Obj(c.id); so != nil && so.Face() != nil && so.Face().Name != "" {
			label = "Apply " + so.Face().Name + "'s replacement"
		} else if dsc := c.repl.Params["Description"]; dsc != "" {
			label = "Apply: " + dsc
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "replacement", Obj: c.id, Label: label})
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
	if len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "madness_") {
		e.handleMadnessReplacement(d, in)
		return
	}
	if len(e.replChoices) == 0 {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement decision answered with no event parked"})
		return
	}
	rc := e.replChoices[0]
	e.replChoices = e.replChoices[1:]
	chosen := d.Chosen(in)
	if len(chosen) == 0 {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement answer had no choice"})
		return
	}
	before := e.triggerBefore
	e.triggerBefore = rc.before
	manaDecision := rc.kind == replChoiceMana || rc.kind == replChoiceManaColor
	switch rc.kind {
	case replChoiceMana:
		if chosen[0].Index < 0 || chosen[0].Index >= len(rc.applicable) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "mana replacement answer out of range"})
			return
		}
		i := rc.applicable[chosen[0].Index]
		if manaReplacementNeedsColor(rc.cands[i]) {
			rc.kind = replChoiceManaColor
			rc.selected = i
			rc.applicable = nil
			e.replChoices = append([]replChoice{rc}, e.replChoices...)
			break
		}
		rc.ev = e.applyOneManaReplacementWithProducer(rc.ev, rc.cands[i], "", rc.manaProducer)
		rc.applied[i] = true
		e.continueManaReplacements(rc.ev, rc.cands, rc.applied, true, rc.manaTapped, rc.manaProducer)
	case replChoiceManaColor:
		color := strings.TrimPrefix(chosen[0].Label, "Add ")
		if len(color) != 1 || !strings.Contains("WUBRG", color) ||
			rc.selected < 0 || rc.selected >= len(rc.cands) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "mana colour replacement answer out of range"})
			return
		}
		rc.ev = e.applyOneManaReplacementWithProducer(rc.ev, rc.cands[rc.selected], color, rc.manaProducer)
		rc.applied[rc.selected] = true
		e.continueManaReplacements(rc.ev, rc.cands, rc.applied, true, rc.manaTapped, rc.manaProducer)
	case replChoicePhaseOrder:
		if chosen[0].Index < 0 || chosen[0].Index >= len(rc.applicable) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "phase replacement answer out of range"})
			return
		}
		i := rc.applicable[chosen[0].Index]
		if rc.cands[i].repl.Params["Optional"] == "True" {
			rc.kind = replChoicePhaseOptional
			rc.selected = i
			rc.applicable = nil
			e.replChoices = append([]replChoice{rc}, e.replChoices...)
		} else {
			e.finishParkedPhase(rc, i)
		}
	case replChoicePhaseOptional:
		if rc.selected < 0 || rc.selected >= len(rc.cands) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "optional phase replacement answer out of range"})
			return
		}
		if chosen[0].Kind == "apply" {
			e.finishParkedPhase(rc, rc.selected)
		} else {
			rc.applied[rc.selected] = true
			e.resumeParkedPhase(rc)
		}
	case replChoiceUntap:
		if chosen[0].Index < 0 || chosen[0].Index >= len(rc.cands) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "untap replacement-order answer out of range"})
			return
		}
		e.applySimpleReplacement(rc.ev, rc.cands[chosen[0].Index])
		if rc.untap != nil && e.pending == nil {
			e.finishUntapStep(rc.untap.next)
		}
	default:
		if chosen[0].Index < 0 || chosen[0].Index >= len(rc.cands) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "replacement-order answer out of range"})
			return
		}
		e.applyReplacement(rc.ev, rc.cands[chosen[0].Index])
	}
	e.triggerBefore = before
	// A mana replacement or colour decision can interrupt CR 601.2g's mana
	// window. Resume the parked cast only after the final rewrite is logged
	// and no next replacement decision is pending.
	if manaDecision && e.pending == nil && len(e.replChoices) == 0 && e.cast != nil {
		e.continueCast()
	}
	e.askNextReplacementChoice()
}

// askNextReplacementChoice hands over to either an ordinary replacement
// competition or a simultaneous Madness choice after the current answer.
func (e *Engine) askNextReplacementChoice() {
	if e.pending != nil {
		return
	}
	if len(e.replChoices) > 0 {
		if p, ok := e.replacementChoicePlayer(e.replChoices[0]); ok {
			e.askReplacementChoice(p)
		}
		return
	}
	if len(e.madnessChoices) > 0 {
		if o := e.G.Obj(e.madnessChoices[0].Obj); o != nil && int(o.Owner) < len(e.G.Players) {
			e.askMadnessReplacement(o.Owner)
		}
	}
}

func (e *Engine) replacementChoicePlayer(rc replChoice) (state.PlayerID, bool) {
	switch rc.kind {
	case replChoiceMana, replChoiceManaColor:
		return rc.ev.Player, int(rc.ev.Player) < len(e.G.Players)
	case replChoicePhaseOrder, replChoicePhaseOptional:
		return e.G.Active, int(e.G.Active) < len(e.G.Players)
	default:
		o := e.G.Obj(rc.ev.Obj)
		if o == nil || int(o.Controller) >= len(e.G.Players) {
			return 0, false
		}
		return o.Controller, true
	}
}

// applyLifeReplacements handles the life-event replacement class without
// changing events.Event: positive LifeChange is a gain, while a negative
// LifeChange and player Damage are life loss. CantGainLife statics and a
// Prevent$ GainLife replacement suppress a gain before it is logged. The
// ReplaceCount$Amount/Twice LifeReduced shape modifies the proposed loss
// before it is logged, which covers Bloodletter and any sibling that uses the
// same Forge replacement expression.
func (e *Engine) applyLifeReplacements(ev events.Event) (events.Event, bool) {
	if ev.Kind == events.LifeChange && ev.Amount > 0 && e.lifeGainPrevented(ev.Player) {
		return e.emit(events.Event{Kind: events.Note, Player: ev.Player, Text: "prevented: cannot gain life"}), true
	}

	p, _, losing := lifeLoss(ev)
	if !losing {
		return ev, false
	}
	forEachReplacement := func(fn func(state.ObjID, *cards.Repl) bool) {
		done := false
		e.forEachObject(func(id state.ObjID) {
			if done {
				return
			}
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				return
			}
			for i := range o.Face().Repls {
				if fn(id, &o.Face().Repls[i]) {
					done = true
					return
				}
			}
		})
	}
	var twice bool
	forEachReplacement(func(source state.ObjID, r *cards.Repl) bool {
		if r.Event != "LifeReduced" || !replacementActive(e, source, r) ||
			!replacementPlayerMatches(e, source, r, p) ||
			!strings.EqualFold(r.Params["PlayerTurn"], "True") || e.G.Active != e.controllerOf(source) {
			return false
		}
		// ReplaceEffect is intentionally not a general API yet. Its one
		// directly evaluable life-reduction expression is structural, not a
		// card-name special case: every future R: line with this exact amount
		// transform is covered by the same branch.
		if r.With == nil || r.With.API != "ReplaceEffect" ||
			!strings.EqualFold(r.With.Params["VarName"], "Amount") ||
			!strings.EqualFold(r.With.Params["VarValue"], "ReplaceCount$Amount/Twice") {
			return false
		}
		twice = true
		return true
	})
	if !twice {
		return ev, false
	}
	if ev.Amount >= -(1<<30) && ev.Amount <= 1<<30 {
		ev.Amount *= 2
	}
	// A replacement applies only once to a proposed event. Re-enter emit so
	// the modified event takes the normal Apply/trigger path, while the guard
	// prevents this same R: line from doubling it again.
	saved := e.applyingReplacement
	e.applyingReplacement = true
	stored := e.emit(ev)
	e.applyingReplacement = saved
	return stored, true
}

// lifeGainPrevented checks active CantGainLife statics and R:Event$ GainLife
// Prevent$ True replacements against the player who would gain life.
func (e *Engine) lifeGainPrevented(p state.PlayerID) bool {
	for _, sv := range e.activeStatics("CantGainLife") {
		if replacementPlayerMatches(e, sv.Source, &cards.Repl{Params: sv.Params}, p) {
			return true
		}
	}
	prevented := false
	e.forEachObject(func(id state.ObjID) {
		if prevented {
			return
		}
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			return
		}
		for i := range o.Face().Repls {
			r := &o.Face().Repls[i]
			if r.Event == "GainLife" && strings.EqualFold(r.Params["Prevent"], "True") &&
				replacementActive(e, id, r) && replacementPlayerMatches(e, id, r, p) {
				prevented = true
				return
			}
		}
	})
	return prevented
}

func replacementActive(e *Engine, source state.ObjID, r *cards.Repl) bool {
	active, ok := r.Params["ActiveZones"]
	if !ok {
		return true
	}
	o := e.G.Obj(source)
	return o != nil && zoneSpecContains(active, o.Zone)
}

func replacementPlayerMatches(e *Engine, source state.ObjID, r *cards.Repl, p state.PlayerID) bool {
	if int(p) >= len(e.G.Players) {
		return false
	}
	v := r.Params["ValidPlayer"]
	return v == "" || effects.MatchesPlayerSpec(e.G, v, p, e.controllerOf(source))
}

func init() {
	// kw:etbCounter and kw:ETBReplacement are implemented wholly by the
	// machinery above: both are R:Event$ Moved replacements (expanded from a
	// K: line by cards/keywords.go) matched and applied here. Reading a card's
	// own tags is what a replacement registration means -- nothing elsewhere
	// in the tree registers them.
	//
	// The four turn/mana replacement events register the same way: repl:Untap
	// (the Basalt Monolith class), repl:BeginPhase (the Necropotence class),
	// repl:Transform (the Sephiroth class) and repl:ProduceMana (the Virtue
	// of Strength class, whose ReplaceWith$ body DB$ ReplaceMana is a
	// registered API). Each is matched by replacementMatches's per-event
	// branch above and applied by applyReplacements's dispatch.
	//
	// The life classes register too: repl:GainLife (the Prevent$ GainLife
	// shape plus the CantGainLife static's replacement arm) and
	// repl:LifeReduced (the ReplaceCount$Amount/Twice shape), both applied by
	// applyLifeReplacements.
	effects.RegisterNonAPI("kw:etbCounter", "kw:ETBReplacement",
		"repl:Untap", "repl:BeginPhase", "repl:Transform", "repl:ProduceMana",
		"repl:GainLife", "repl:LifeReduced")
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
