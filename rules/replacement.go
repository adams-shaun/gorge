// replacement.go applies R: line replacement effects ahead of an event's
// own logging, from engine.go's emit. applyReplacements discovers every
// applicable effect in deterministic order, then applies the event-specific
// CR 616 ordering and optional-effect rules.
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
	// Positive LifeChange is a gain; it never carries a repl:DamageDone
	// match (that class names a Damage event only), so it routes straight
	// to the life replacement machinery.
	if ev.Kind == events.LifeChange {
		return e.applyLifeReplacements(ev)
	}
	if ev.Kind == events.Damage && ev.Obj == 0 {
		// Player-targeted damage is CR 616-eligible for TWO competing
		// classes: repl:DamageDone (Battletide Alchemist's own damage-
		// specific prevention) and the general repl:LifeReduced life-loss
		// machinery (CR 615's "the next time a player would lose life" —
		// damage-caused loss counts). DamageDone is the more specific class
		// and is tried first through the ordinary dispatch below; only when
		// nothing there applies (or a DamageDone ReplaceEffect body rewrote
		// the amount in place without fully replacing the event) does the
		// event fall through to applyLifeReplacements, which also
		// recognises damage-caused loss (lifeLoss/trigger_match.go). A card
		// with both classes active competing for the SAME event is not in
		// the corpus this build measures against; that composition is left
		// for whichever ticket first needs it.
		replaced, handled := e.applyReplacementsDispatch(ev)
		if handled {
			return replaced, true
		}
		return e.applyLifeReplacements(replaced)
	}
	return e.applyReplacementsDispatch(ev)
}

// applyReplacementsDispatch is applyReplacements' common match-collection
// and per-event dispatch, shared by every replacement-eligible event kind
// (Moved/Untap/BeginPhase/Transform/ProduceMana/DamageDone). Factored out so
// applyReplacements can wrap a player-targeted Damage event with the
// repl:LifeReduced fallback above without duplicating this body.
func (e *Engine) applyReplacementsDispatch(ev events.Event) (events.Event, bool) {
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
	// Riot (CR 702.108) asks its counter-or-haste question as the creature
	// would enter; parking the move keeps the entry out of the log until the
	// as-enters choice is recorded next to it.
	if e.applyRiotReplacement(ev) {
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
	// Effect-created replacements (Blood of the Martyr and the broader
	// ReplacementEffects$ family) are active independently of their source's
	// current zone. active() enforces the Effect duration; reconstruct the
	// Forge R: body into the same replMatch path used by printed replacements
	// so filters, ordering and replacement context cannot drift.
	for _, ce := range e.active() {
		if ce.ReplacementEvent == "" || ce.ReplacementEvent != event {
			continue
		}
		if with := replacementBodySA(ce.ReplacementBody); with != nil {
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: with}
			if e.replacementMatchesEffectCreated(*r, ce.Source, ev, ce.Remembered) {
				matches = append(matches, replMatch{id: ce.Source, repl: r, remembered: ce.Remembered,
					chosen: ce.ChosenNumber,
					key:    "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))})
			}
		} else if ce.ReplacementBody == "" && (strings.EqualFold(strings.TrimSpace(ce.ReplacementParams["Layer"]), "CantHappen") ||
			(event == "DamageDone" && strings.EqualFold(ce.ReplacementParams["Prevent"], "True"))) {
			// The Effect-created CantHappen form (Mistrise Village's AntiMagic:
			// "the next spell you cast this turn can't be countered"): no
			// ReplaceWith$ — stopping the event is the complete replacement,
			// the same shape printed R: lines take (the With==nil arm below).
			// The bodyless Prevent$ True DamageDone form (Selfless Squire's
			// RPrevent, task dponce1; the wider bodyless prevent family it
			// belongs to) is the same idiom for damage: full prevention is the
			// complete replacement, applied by the shared damage dispatch
			// (applyNonMoveReplacements' Prevent$ arm) exactly as a printed R:
			// line's would be.
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams}
			if e.replacementMatchesEffectCreated(*r, ce.Source, ev, ce.Remembered) {
				matches = append(matches, replMatch{id: ce.Source, repl: r, remembered: ce.Remembered,
					key: "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))})
			}
		}
	}
	e.forEachReplacementSource(func(id state.ObjID) {
		f := e.replacementFace(id, ev)
		if f == nil {
			return
		}
		for i := range f.Repls {
			if !replacementEventNameMatches(f.Repls[i].Event, event) {
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
	case events.TokenCreate:
		return e.continueCreateTokenReplacements(ev, matches)
	case events.Damage:
		matches = e.applicableDamageReplacements(ev, matches)
		if len(matches) == 0 {
			return ev, false
		}
		if len(matches) > 1 || hasOptionalReplacement(matches) {
			if p, ok := e.damageAffectedPlayer(ev); ok && !e.G.Players[p].Lost {
				e.poseDamageReplacementChoice(ev, matches, e.replacementAskPlayer(matches, p))
				return events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
					Text: "damage awaiting replacement-order choice"}, true
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
		// against the changed event (not the original amount). The recheck
		// uses the same matcher class the collection used (see
		// applicableDamageReplacements): an Effect-created match is never
		// re-gated on ActiveZones$.
		matched := false
		if m.key != "" {
			matched = e.replacementMatchesEffectCreated(*m.repl, m.id, ev, m.remembered)
		} else {
			matched = e.replacementMatches(*m.repl, m.id, ev)
		}
		if !matched {
			continue
		}
		if ev.Kind == events.Damage && damageReplacementPrevents(*m.repl) {
			if e.cantPreventDamage(e.damaging, ev.Obj) {
				// Neither a Prevent$ True line nor a DB$ ReplaceDamage body
				// may touch damage that cannot be prevented (Spider-Punk).
				continue
			}
			if strings.EqualFold(m.repl.Params["Prevent"], "True") {
				// Stored through a re-entrant emit (the ReplaceDamage arm's
				// shape, task dponce1): the log record IS the prevention's
				// occurrence, so Mode$ DamagePreventedOnce triggers fire off it
				// -- Amount carries the prevented damage (Note is an Apply
				// no-op marker; no reader of Note.Amount predates this). Obj is
				// the damaged object (0 for a player hit) and Player the
				// damaged player, the uniform shape every stored prevention
				// Note keeps. The returned kind is still a Note, never a
				// Damage: the combat assignment loop and speed.go read it.
				return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
					Amount: ev.Amount, Text: "damage prevented by replacement effect"}), true
			}
			// a ReplaceDamage body falls through to its subtracting arm below
		}
		if m.repl.With == nil {
			// Counter's Layer$ CantHappen shape has no ReplaceWith$: stopping
			// the event is its complete replacement.
			return ev, true
		}
		if m.repl.With.API == "ReplaceDamage" {
			// Handled here, not through runReplaceWith/effects.Resolve: the
			// body subtracts its Amount from the held event and reports
			// terminal when fully prevented (its prevention Note is the log's
			// record, exactly as a Prevent$ True match's) or leaves the
			// reduced event standing for the next modifier. Its SubAbility$
			// chain is deliberately not run (see applyReplaceDamageBody).
			if e.applyReplaceDamageBody(&ev, m) {
				return ev, true
			}
			continue
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

// damageReplacementPrevents reports whether this replacement is a
// PREVENTION body: either the legacy Prevent$ True shape or a DB$
// ReplaceDamage body, which subtracts its Amount from the held damage event
// and prevents exactly that much (the Thunderstaff/Battletide shield
// family). stat:CantPreventDamage must exclude BOTH shapes, so every
// damage-replacement selection and application path classifies prevention
// through this one predicate and cannot drift apart.
func damageReplacementPrevents(r cards.Repl) bool {
	// Case-insensitive (dponce1 r2): the registration (effEffect's
	// replacementLinePrevents) and the collection
	// (applyReplacementsDispatch) read the param with EqualFold, so this
	// classifier must too — a non-canonical `Prevent$ true` bodyless
	// registration would otherwise be admitted to the competition and then
	// silently erased by the With==nil CantHappen drop arm.
	if strings.EqualFold(r.Params["Prevent"], "True") {
		return true
	}
	return r.With != nil && r.With.API == "ReplaceDamage"
}

func (e *Engine) applicableDamageReplacements(ev events.Event, matches []replMatch) []replMatch {
	out := matches[:0]
	for _, m := range matches {
		// CR 616.1's recheck must use the same matcher class the initial
		// collection used: an Effect-created match's lifetime is active()'s,
		// not its source's zone (task wildgrowth1), so re-gating it on
		// ActiveZones$ here would silently drop every Effect-granted
		// DamageDone replacement the scan just admitted (Taii Wakeen).
		matched := false
		if m.key != "" {
			matched = e.replacementMatchesEffectCreated(*m.repl, m.id, ev, m.remembered)
		} else {
			matched = e.replacementMatches(*m.repl, m.id, ev)
		}
		if !matched {
			continue
		}
		if damageReplacementPrevents(*m.repl) && e.cantPreventDamage(e.damaging, ev.Obj) {
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
	face *cards.Face // prospective face for an "as this transforms" replacement
	repl *cards.Repl
	// key identifies an Effect-created replacement across active() rebuilds.
	// Printed replacement pointers are immutable face entries and need no key.
	key string
	// remembered carries the Effect-created replacement's remembered ids so
	// a per-mint re-match (continueCreateTokenReplacements) can re-evaluate
	// its IsRemembered specs exactly as the initial match did. Printed
	// replacements never carry one.
	remembered []state.ObjID
	// chosen carries the Effect-created replacement's SetChosenNumber$ binding
	// (state.ContinuousEffect.ChosenNumber, task wildgrowth1): the number the
	// Effect resolved at creation, which replCtx threads into the body Ctx so
	// the body's Count$ChosenNumber head reads the frozen binding. Zero on
	// every printed replacement (and on an Effect that bound nothing).
	chosen int32
}

// rememberedSpecContext builds the match context a ValidCard$/ValidLKI$
// spec on a Moved replacement evaluates under: the ordinary You/Source pair,
// plus the remembered ids as targets when the caller carries any (the
// Effect-created ReplaceDyingDefined$ family). Nil ids yield the plain
// context every other caller already built.
func (e *Engine) rememberedSpecContext(you state.PlayerID, source state.ObjID, remembered []state.ObjID) effects.SpecContext {
	sc := effects.SpecContext{You: you, Source: source}
	if len(remembered) > 0 {
		for _, id := range remembered {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: id})
		}
	}
	return sc
}

func hasOptionalReplacement(matches []replMatch) bool {
	for _, m := range matches {
		if strings.EqualFold(m.repl.Params["Optional"], "True") {
			return true
		}
	}
	return false
}

// replacementOptionalDecider resolves the controller an Optional$ True
// replacement's "may" belongs to, from Forge's OptionalDecider$ -- named in
// the replacement source's frame. The corpus's two Optional$ DamageDone lines
// (Blood of the Martyr, Battletide Alchemist) both say "You": the source's
// controller, not the damaged player. An ABSENT parameter resolves to nobody
// (the caller keeps its historical default); a present spec this build does
// not resolve also resolves to nobody, and the match gate above excluded the
// replacement already, so this is only reachable for "You".
func (e *Engine) replacementOptionalDecider(r cards.Repl, source state.ObjID) (state.PlayerID, bool) {
	if strings.TrimSpace(r.Params["OptionalDecider"]) != "You" {
		return 0, false
	}
	ctrl := e.controllerOf(source)
	if ctrl < 0 || int(ctrl) >= len(e.G.Players) {
		return 0, false
	}
	return ctrl, true
}

// replacementAskPlayer picks who answers a damage replacement competition.
// The CR 616.1 order choice belongs to the affected player -- except that a
// competition of exactly ONE Optional$ True replacement belongs to that
// replacement's OptionalDecider$ (Blood of the Martyr and Battletide
// Alchemist both name "You": their own controller), because then the only
// question posed is the optional replacement's own "may", not an order.
// A decider who has lost or left makes no choices (CR 800.4a), so the
// affected player answers instead.
func (e *Engine) replacementAskPlayer(matches []replMatch, affected state.PlayerID) state.PlayerID {
	if len(matches) == 1 && strings.EqualFold(matches[0].repl.Params["Optional"], "True") {
		if dp, ok := e.replacementOptionalDecider(*matches[0].repl, matches[0].id); ok &&
			!e.G.Players[dp].Lost {
			return dp
		}
	}
	return affected
}

// replaceDamageAmount resolves a DB$ ReplaceDamage body's Amount$ in the
// replacement source's context: the corpus's prevention-shield family prices
// it with a literal (Thunderstaff's 1), an SVar (Battletide Alchemist's
// AlchemicX, the card-defined ShieldAmount of Forcefield's "prevent all but
// 1") or an inline expression. The bool distinguishes an unresolvable value
// frame (Power Leak's PaidAmount) -- which the match gate turns into a
// non-match -- from a resolvable amount of zero, which legitimately prevents
// nothing.
func (e *Engine) replaceDamageAmount(ev events.Event, m replMatch) (int32, bool) {
	if m.repl.With == nil || m.repl.With.API != "ReplaceDamage" {
		return 0, false
	}
	return effects.NumResolved(e, e.replCtx(m, ev), m.repl.With, "Amount", 0)
}

// applyReplaceDamageBody applies a DB$ ReplaceDamage body to the held damage
// event: it subtracts the body's Amount from the event's remaining amount and
// reports whether the event is TERMINAL (fully prevented -- the prevention
// Note this records is the log's witness, and no reduced Damage event is
// emitted) or still stands with its reduced amount for the next modifier in
// the chain (CR 616.1e: each later opportunity reads the changed event). A
// resolvable amount of zero prevents nothing and leaves the event standing.
// The body's SubAbility$ chain is deliberately NOT run here: every corpus
// body that carries one (Divine Deflection's counter-deal, Forcefield's
// self-exile) is an Effect-shield shape whose sub reads bindings this
// per-event application does not have, and running them would fire a
// wrong-outcome rider -- the prevention itself is the correct core.
func (e *Engine) applyReplaceDamageBody(ev *events.Event, m replMatch) bool {
	n, ok := e.replaceDamageAmount(*ev, m)
	if !ok || n <= 0 {
		return false
	}
	prevented := n
	if prevented > ev.Amount {
		prevented = ev.Amount
	}
	ev.Amount -= prevented
	who := "a replacement effect"
	if o := e.G.Obj(m.id); o != nil && o.Face() != nil && o.Face().Name != "" {
		who = o.Face().Name
	}
	e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
		Amount: prevented,
		Text:   who + " prevented " + strconv.Itoa(int(prevented)) + " of the damage"})
	// Obj carries the damaged object (0 for a player hit) like the full-
	// prevention arm's Note above, not the preventing source: the log text
	// names the preventer, and Mode$ DamagePreventedOnce triggers key their
	// ValidTarget$ on the damaged side. (Prevention Notes carried no Amount
	// before dponce1 and zero prevention-text events are logged in the
	// golden-shape games, so the field's presence is stream-neutral there.)
	return ev.Amount <= 0
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

// replacementEventNameMatches compares a printed R:Event$ name with the
// event name an engine event maps to. "DrawCards" is Forge's spelling of
// the draw replacement event (Quantum Riddler's "you draw that many cards
// plus one instead", Alms Collector's "if an opponent would draw two or
// more cards"); the engine maps events.Draw to "Draw", so the alias reads
// here rather than in the IR (two corpus carriers, both
// CheckSVar$/Number$-gated).
func replacementEventNameMatches(replEvent, event string) bool {
	if replEvent == event {
		return true
	}
	return event == "Draw" && replEvent == "DrawCards"
}

// drawMatchAmount is the replacement-CONTEXT amount of a Draw event: a
// per-card Draw carries no Amount, and the body view of "the number of cards
// this draw would draw" is 1 (Quantum Riddler's NumCards$
// ReplaceCount$Number/Plus.1). The EMITTED event is never touched -- this is
// a context read only -- which is what keeps every unrelated game's Draw
// events byte-identical.
func drawMatchAmount(ev events.Event) int32 {
	if ev.Kind == events.Draw && ev.Amount == 0 {
		return 1
	}
	return ev.Amount
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
	case events.Damage:
		return "DamageDone", true
	case events.Draw:
		return "Draw", true
	case events.TokenCreate:
		return "CreateToken", true
	default:
		return "", false
	}
}

// replacementFace returns the source face whose R: lines apply now. A
// transform's "as this transforms into ..." replacement belongs to the
// destination face, while every other replacement reads the source's current
// face. This avoids making the alternate face live for unrelated events.
func (e *Engine) replacementFace(id state.ObjID, ev events.Event) *cards.Face {
	o := e.G.Obj(id)
	if o == nil || o.Card == nil {
		return nil
	}
	if ev.Kind == events.FlipFace && id == ev.Obj && ev.Amount >= 0 && int(ev.Amount) < len(o.Card.Faces) {
		return o.Card.Faces[ev.Amount]
	}
	return o.Face()
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
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, nil)
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
		e.runReplaceWith(e.replCtx(m, ev), 0, m.repl.With, nil)
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
			e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, nil)
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
			e.loop.observe(stored)
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
	e.runReplaceWith(ctx, e.manaProducer, m.repl.With, nil)
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
// the {X} paid for the moving object, Replaced names the object the replaced
// event was about (so a Defined$ ReplacedCard body finds its subject), and
// the SVar table comes from the source's face. This is exactly the context
// the single-match path built inline for M1; it is factored out so the
// composition and order-choice paths reuse it.
//
// Ctx.Remembered deliberately starts EMPTY. The replaced object is reached
// through Replaced/Defined$ ReplacedCard, never through Remembered: every
// corpus body that counts Remembered$Amount inside a replacement (the
// pay-before-ETB family's ConditionCheckSVar$ X gates — Mox Diamond, Scorched
// Ruins, Soldevi Excavations, the sac-lands — and the CounterNum$ reads)
// rides its own Remember* rider (RememberDiscarded$/RememberSacrificed$/
// RememberChanged$/RememberRevealed$), and seeding the replaced card here
// made every one of those counts one too high (measured over the compiled
// corpus: all nine gating bodies carry a rider; none reads the seed). The
// seed was fx44's stand-in for the suspended-resolution case; the resume
// thread carries the body's own Remembered instead (rules/resolution.go's
// replacement branch).
//
// The rule is scoped to PRINTED replacements. An EFFECT-created replacement
// (m.key != "", task wildgrowth1) is a different population: the effect's
// registered Remembered list IS the intended binding -- the trigger captured
// the cast spell with RememberObjects$ and the body's Remembered$ specs (the
// runadi_behemoth_caller / gluttonous_hellkite CounterNum$ readers) read
// exactly that -- and none of those bodies rides a competing Remember* rider,
// so the double-counting defect the printed rule exists for cannot arise.
// replCtx threads the effect's frozen SetChosenNumber$ binding alongside it
// (replMatch.chosen), what the body's Count$ChosenNumber head reads.
func (e *Engine) replCtx(m replMatch, ev events.Event) *effects.Ctx {
	o := e.G.Obj(m.id)
	target := state.Target{Obj: ev.Obj}
	if ev.Kind == events.Damage && ev.Obj == 0 {
		target = state.Target{Player: ev.Player, IsPlayer: true}
	}
	if o == nil {
		ctx := &effects.Ctx{Source: m.id, ReplacementTarget: target,
			ReplacementSource: e.protectionSource(e.damaging), ReplacementAmount: drawMatchAmount(ev)}
		e.seedEffectReplCtx(ctx, m)
		return ctx
	}
	ctx := &effects.Ctx{Source: m.id, Controller: o.Controller,
		ReplacementTarget: target, ReplacementSource: e.protectionSource(e.damaging),
		ReplacementAmount: drawMatchAmount(ev),
		// X is the {X} paid for the moving object, so an ETB replacement that
		// reads it (etbCounter's CounterNum$ X, e.g. Endless One / Walking
		// Ballista / Chalice of the Void) sees the value the player actually
		// chose. Move preserves X from the stack onto the permanent (events/
		// apply.go, the "hand/stack -> battlefield must NOT reset them"
		// comment), so o.X is the cast-time value here.
		X:        o.X,
		Captured: []state.Target{{Obj: ev.Obj}},
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
	if ev.Kind == events.Draw {
		// A replaced DRAW names the draw-er, not (only) the card: the body's
		// ReplacedPlayer selectors (UnlessPayer$, Defined$) resolve against
		// this, and its own DB$ Draw re-does the draw the original event
		// would have done, so the Remembered/Captured seed above is dropped
		// for draws — the body's own RememberDrawn$ records what it actually
		// drew (a seeded stale entry would double the reveal's and the
		// discard condition's population).
		ctx.ReplacedPlayer = state.Target{Player: ev.Player, IsPlayer: true}
		ctx.Remembered, ctx.Captured = nil, nil
	}
	if f != nil {
		effects.SetSVars(ctx, f.SVars)
	}
	e.seedEffectReplCtx(ctx, m)
	return ctx
}

// seedEffectReplCtx threads an Effect-created match's own bindings into the
// body Ctx: the frozen SetChosenNumber$ number and (key-only: never a printed
// replacement) the effect's registered Remembered list. See replCtx's doc for
// why the seed is scoped to effect-created matches. The Draw branch above
// clears ctx.Remembered for the bodies that re-draw; no Draw body is ever
// registered live (effects' registration gate keeps them on the loud Note),
// so a live seed cannot be erased by that clearing on a registered shape.
func (e *Engine) seedEffectReplCtx(ctx *effects.Ctx, m replMatch) {
	ctx.ChosenNumber = m.chosen
	// The bound flag is the Count$ChosenNumber head's verdict: effect-created
	// only, so a printed or choose-event context stays UNRESOLVED and the
	// EvalCountOK consumers keep their fail direction (see Ctx.ChosenNumberBound).
	ctx.ChosenNumberBound = m.key != ""
	if m.key == "" || len(m.remembered) == 0 {
		return
	}
	for _, id := range m.remembered {
		ctx.Remembered = append(ctx.Remembered, state.Target{Obj: id})
	}
}

// runReplaceWith resolves one ReplaceWith$ body inside the replacement
// re-entrancy guard (CR 616.1, a replacement applies once), recording the
// replaced object for the body's Defined$/Remembered$ reads and restoring
// both the guard and that record afterward so an outer replacement keeps its
// own state.
// runReplaceWith resolves one ReplaceWith$ body inside the applyingReplacement
// guard, recording the state a nested emit needs: replReplaced/replAction
// (Engine.emit's events.CarryAction, so a body's own move of replReplaced
// carries the replaced event's action marker), replacingEvent/replacingSource
// (ReplaceEvent's target -- a DB$ ReplaceDamage body reaching back to rewrite
// the live Damage event's Amount/Affected fields), and replReplacedPlayer (the
// draw-er a Draw replacement's body's ReplacedPlayer selectors read on
// resume). ev is nil wherever the original event was already logged (the
// "Updated" shape) or has no action marker worth carrying (mana/ETB
// replacements); passing it derives the action automatically rather than
// making every caller compute it.
func (e *Engine) runReplaceWith(ctx *effects.Ctx, replaced state.ObjID, with *cards.SA, ev *events.Event) {
	savedRepl, savedEvent, savedSource, savedAction, savedPlayer :=
		e.replReplaced, e.replacingEvent, e.replacingSource, e.replAction, e.replReplacedPlayer
	e.applyingReplacement = true
	action := ""
	if ev != nil {
		action = events.ActionMarker(*ev)
	}
	e.replReplaced, e.replacingEvent, e.replacingSource, e.replAction, e.replReplacedPlayer =
		replaced, ev, ctx.Source, action, ctx.ReplacedPlayer
	e.resolveReplacementWith(ctx, with)
	e.replReplaced, e.replacingEvent, e.replacingSource, e.replAction, e.replReplacedPlayer =
		savedRepl, savedEvent, savedSource, savedAction, savedPlayer
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
		e.loop.observe(stored)
		// The move-driven Effect lifetimes (the ExileOnMoved$/ForgetOnMoved$
		// sweep) run on Engine.emit's own MoveZone path right here in the
		// ordering; the raw events.Emit above bypasses that path, so the sweep
		// is replayed inline (task wildgrowth1: the "enters with N additional
		// counters" Effect must end exactly after the one entry it upgraded,
		// not linger to re-upgrade the same remembered card's next entry).
		e.effectMoveSweep(ev)
		e.checkTriggers(stored, nil, 0, 0, false)
		e.finishSourceLifelinkLKI(ev, departing, link, controller)
		e.runReplaceWith(ctx, ev.Obj, m.repl.With, nil)
		return stored, true
	}
	e.runReplaceWith(ctx, ev.Obj, m.repl.With, &ev)
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
	e.loop.observe(stored)
	// The move-driven Effect lifetimes, replayed inline exactly as the
	// single-match Updated branch does (the raw events.Emit above bypasses
	// Engine.emit's own sweep point).
	e.effectMoveSweep(ev)
	e.checkTriggers(stored, nil, 0, 0, false)
	e.finishSourceLifelinkLKI(ev, departing, link, controller)
	for _, m := range matches {
		if m.repl.With == nil {
			continue
		}
		e.runReplaceWith(e.replCtx(m, ev), ev.Obj, m.repl.With, nil)
	}
	return stored, true
}

// continueCreateTokenReplacements applies every applicable CreateToken
// replacement to one TokenCreate event. The engine mints ONE token per
// event, so the plan starts as that single mint. Each match applies at most
// once, in deterministic scan order, and its ValidToken$ gate is re-checked
// PER PLAN MINT — a later match sees the mints earlier matches produced
// (Divine Visitation after a doubler replaces each doubled mint that is
// still a creature token), which is CR 616.1's re-application over the
// changed event. The final plan is emitted directly through events.Emit
// (+ observe + checkTriggers per mint, the composeUpdatedReplacements
// pattern), which BYPASSES applyReplacements: no mint can re-match, so a
// doubler can never loop on its own output. The deviation from CR 616.1 is
// deliberate and documented: competing CreateToken replacements apply in
// scan order, NOT through a posed KReplacement order choice (non-commuting
// compositions are reachable in Commander, but no repo deck carries any of
// this family, so no golden game exercises one).
func (e *Engine) continueCreateTokenReplacements(ev events.Event, matches []replMatch) (events.Event, bool) {
	plan := []string{ev.Text}
	for _, m := range matches {
		body := m.repl.With
		if body == nil || body.API != "ReplaceToken" {
			// A body this dispatcher does not read leaves the plan untouched;
			// the mint stands (the fail-safe direction).
			continue
		}
		if strings.EqualFold(m.repl.Params["Optional"], "True") {
			// The deterministic decline stand-in (the optional no-ask paths'
			// contract): a "may" replacement with no chooser applies as if
			// declined, the event stands verbatim.
			continue
		}
		typ := strings.TrimSpace(body.Params["Type"])
		if typ == "ReplaceController" {
			e.emit(events.Event{Kind: events.Note, Obj: m.id, Player: ev.Player,
				Text: "ReplaceToken Type$ ReplaceController is not implemented; the token is created unchanged"})
			continue
		}
		if strings.EqualFold(strings.TrimSpace(body.Params["TokenScript"]), "Chosen") ||
			strings.TrimSpace(body.Params["ValidChoices"]) != "" {
			e.emit(events.Event{Kind: events.Note, Obj: m.id, Player: ev.Player,
				Text: "ReplaceToken ValidChoices (TokenScript$ Chosen) is not implemented; the token is created unchanged"})
			continue
		}
		plan = e.applyTokenReplacementToPlan(ev, plan, m)
	}
	if len(plan) == 1 && plan[0] == ev.Text {
		// No replacement changed the plan: the ordinary emit path logs the
		// original event untouched (with its full LKI/trigger treatment).
		return ev, false
	}
	if len(plan) == 0 {
		// Every mint was removed (halving_season's HalfDown on a one-token
		// event): the Note is the log's witness that nothing was created.
		e.emit(events.Event{Kind: events.Note, Obj: 0, Player: ev.Player,
			Text: "no tokens created (replacement effect rounded the creation down to zero)"})
		return ev, true
	}
	var last events.Event
	for _, script := range plan {
		mint := events.Event{Kind: events.TokenCreate, Player: ev.Player, Text: script}
		stored := events.Emit(e.G, e.L, mint)
		e.loop.observe(stored)
		e.checkTriggers(stored, nil, 0, 0, false)
		last = stored
	}
	return last, true
}

// applyTokenReplacementToPlan transforms the plan by ONE match, per mint,
// with the match's own ValidToken$ re-checked against each mint's script.
func (e *Engine) applyTokenReplacementToPlan(ev events.Event, plan []string, m replMatch) []string {
	body := m.repl.With
	switch strings.TrimSpace(body.Params["Type"]) {
	case "ReplaceToken":
		// "... instead create those tokens as <scripts>" — a pure rewrite:
		// each matched mint is replaced by one mint per script in the CSV
		// (Academy Manufactor's one Clue -> Clue+Food+Treasure; Divine
		// Visitation's squirrel -> angel).
		scripts := e.knownTokenScripts(m.id, body.Params["TokenScript"])
		if len(scripts) == 0 {
			return plan
		}
		out := make([]string, 0, len(plan)*len(scripts))
		for _, mint := range plan {
			if e.tokenReplacementMatchesMint(ev, m, mint) {
				out = append(out, scripts...)
			} else {
				out = append(out, mint)
			}
		}
		return out
	case "AddToken":
		// "... instead create those tokens plus N <script>" — the original
		// mint stands and N extra mints of the named script join it.
		n := int32(1)
		if raw := strings.TrimSpace(body.Params["Amount"]); raw != "" {
			v, ok := tokenReplaceCount(raw)
			if !ok || v < 0 {
				e.emit(events.Event{Kind: events.Note, Obj: m.id, Player: ev.Player,
					Text: "ReplaceToken Amount$ " + raw + " is not implemented; the token is created unchanged"})
				return plan
			}
			n = v
		}
		extra := e.knownTokenScripts(m.id, body.Params["TokenScript"])
		if len(extra) == 0 {
			return plan
		}
		out := make([]string, 0, len(plan)+int(n)*len(extra))
		for _, mint := range plan {
			out = append(out, mint)
			if e.tokenReplacementMatchesMint(ev, m, mint) {
				for i := int32(0); i < n; i++ {
					out = append(out, extra...)
				}
			}
		}
		return out
	default:
		// "Amount" (and an absent Type$ — the corpus always names one, Amount
		// is the natural default for the "twice that many" doubler family):
		// each matched mint becomes replCountOp(1, Amount$) copies of itself.
		raw := strings.TrimSpace(body.Params["Amount"])
		if raw == "" {
			raw = "Twice"
		}
		n, ok := tokenReplaceCount(raw)
		if !ok || n < 0 {
			e.emit(events.Event{Kind: events.Note, Obj: m.id, Player: ev.Player,
				Text: "ReplaceToken Amount$ " + raw + " is not implemented; the token is created unchanged"})
			return plan
		}
		out := make([]string, 0, len(plan)*int(n)+1)
		for _, mint := range plan {
			if !e.tokenReplacementMatchesMint(ev, m, mint) {
				out = append(out, mint)
				continue
			}
			if n >= 1 {
				for i := int32(0); i < n; i++ {
					out = append(out, mint)
				}
				continue
			}
			// n == 0 (HalfDown against this engine's one-token events): the
			// matched mint is not created. A halving composed AFTER a
			// multiplier applies per mint rather than to the plan total — the
			// documented composition approximation (see AGENTS.md).
		}
		return out
	}
}

// tokenReplacementMatchesMint re-checks ONE replacement against ONE plan
// mint (a would-be TokenCreate event over the mint's script). The body's
// own ValidCard$ (stridehangar_automaton's redundant artifact gate) joins
// the gate when present.
func (e *Engine) tokenReplacementMatchesMint(ev events.Event, m replMatch, script string) bool {
	mint := events.Event{Kind: events.TokenCreate, Player: ev.Player, Text: script}
	// The recheck uses the same matcher class the initial collection used:
	// an effect-created match re-matches through the remembered-scoped effect
	// matcher, a printed match through the ordinary one -- never the ungated
	// effect matcher for a printed key. For a TokenCreate mint the two agree
	// on every printed Repl today (the mint, like ev, carries no To), but the
	// split keeps the recheck from ever widening what the initial gated
	// collection admitted, the same discipline remainingDamageReplacements
	// and counterReplacementMatchesAll follow.
	if m.key != "" {
		if !e.replacementMatchesEffectCreated(*m.repl, m.id, mint, m.remembered) {
			return false
		}
	} else if !e.replacementMatches(*m.repl, m.id, mint) {
		return false
	}
	if m.repl.With != nil {
		if v := strings.TrimSpace(m.repl.With.Params["ValidCard"]); v != "" {
			tok := e.tokenSnapshot(mint)
			if tok == nil || !effects.MatchesObjectCtx(e.G, v, tok,
				e.rememberedSpecContext(e.controllerOf(m.id), m.id, m.remembered)) {
				return false
			}
		}
	}
	return true
}

// knownTokenScripts splits a ReplaceToken body's TokenScript$ CSV and keeps
// only the stems the game's token registry knows, one loud Note per unknown
// stem. An empty result leaves the caller's plan untouched.
func (e *Engine) knownTokenScripts(source state.ObjID, csv string) []string {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := e.G.Tokens[s]; !ok {
			e.emit(events.Event{Kind: events.Note, Obj: source,
				Text: "unknown token script " + s})
			continue
		}
		out = append(out, s)
	}
	return out
}

// tokenReplaceCount resolves a ReplaceToken body's Amount$: a literal
// integer (the AddToken family's Amount$ 1) or one of replCountOp's word
// ops read against the single-mint base (Twice -> 2, Thrice -> 3, HalfDown
// -> 0 against the one-token event this engine mints — replCountOp is the
// shared word-op parser). An unresolvable value (X, an SVar name) reports
// not-ok and the match is dropped with a loud Note.
func tokenReplaceCount(raw string) (int32, bool) {
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), true
	}
	switch {
	case raw == "Twice", raw == "Thrice", raw == "HalfDown", raw == "HalfUp",
		strings.HasPrefix(raw, "Plus."), strings.HasPrefix(raw, "Minus."),
		strings.HasPrefix(raw, "Times."):
		return replCountOp(1, raw), true
	}
	return 0, false
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

// applyRiotReplacement parks every non-cast battlefield entry of a Riot
// creature before it happens. Cast flow already records RiotChoice through
// collectETBChoices, but reanimation/blink/search entries only visit this
// general MoveZone path. The parked move is emitted after handleChoose logs
// the choice, making events.Move the single place that applies it.
func (e *Engine) applyRiotReplacement(ev events.Event) bool {
	// A battlefield entry reached while another decision is outstanding must
	// not overwrite it (the orphaned-pending failure poseLifeReplacementChoice
	// already guards, and the same class findings-sol4 proved on the
	// life-replacement draw loop): let the entry happen verbatim rather than
	// parking on an ask that can never be posed. Unreachable in the corpus
	// today; the guard keeps a future caller from shipping the overwrite.
	if ev.To != state.ZBattlefield || e.riotMove != nil || e.pending != nil {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone == state.ZBattlefield || o.Face() == nil ||
		!o.Face().HasKeyword("Riot") || o.RiotChoice != "" {
		return false
	}
	move := ev
	e.riotMove = &move
	d := &decision.Decision{Player: o.Controller, Kind: decision.KChoose, Min: 1, Max: 1,
		Source: o.ID, Prompt: "Choose how this creature enters (counter or haste)",
		Options: []decision.Option{
			{Index: 0, Kind: "riot", Label: "Enter with a +1/+1 counter", Obj: o.ID, Player: o.Controller},
			{Index: 1, Kind: "riot", Label: "Gain haste", Obj: o.ID, Player: o.Controller},
		}}
	e.choosing = chooseRiot
	e.ask(d)
	return true
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
	return e.replacementMatchesRemembered(r, source, ev, nil)
}

// replacementMatchesRemembered is replacementMatches with an optional
// remembered set: the remembered ids an Effect-created replacement carries
// (DealDamage's ReplaceDyingDefined$ registration) are what its ValidCard$/
// ValidLKI$ Card.IsRemembered spec is matched against — the Effect captured
// them when it resolved, and its continuous registry entry is the only place
// that set still lives. Printed R: lines pass nil and never see a remembered
// binding; a spec carrying IsRemembered against an empty set fails closed,
// the matcher's standing contract.
func (e *Engine) replacementMatchesRemembered(r cards.Repl, source state.ObjID, ev events.Event, remembered []state.ObjID) bool {
	if !e.activeZonesGateOK(r, source, ev) {
		return false
	}
	return e.replacementMatchesRememberedUngated(r, source, ev, remembered)
}

// activeZonesGateOK is the ActiveZones$ zone gate the PRINTED replacement
// paths share:
//
//   - the generic object walk historically excludes the command zone; its
//     replacement-only extension admits command-zone sources, but only when
//     the script explicitly declares that zone; this keeps ordinary card
//     text parked there inert and does not change trigger discovery.
//
//   - CR 611.3b/614.4: a static replacement only applies from one of its
//     declared active zones. Accept the comma-separated list grammar used by
//     other Forge zone parameters; the pinned corpus currently uses only
//     singleton ActiveZones values. Replacements with no ActiveZones
//     parameter apply from anywhere (the corpus does not thereby declare a
//     zone, and historically this engine has allowed those from anywhere).
//
//   - A permanent's own entry replacement is active for the event that puts
//     it into the declared zone even though the source has not arrived there
//     yet (CR 614.12): the prospective ev.To clause. ev.To is only
//     meaningful for a MoveZone; the other events leave it at its zero
//     value, so the clause is MoveZone-only rather than reading a
//     meaningless zero zone.
func (e *Engine) activeZonesGateOK(r cards.Repl, source state.ObjID, ev events.Event) bool {
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZCommand {
		active, ok := r.Params["ActiveZones"]
		if !ok || !zoneSpecContains(active, state.ZCommand) {
			return false
		}
	}
	if active, ok := r.Params["ActiveZones"]; ok {
		o := e.G.Obj(source)
		currentlyActive := o != nil && zoneSpecContains(active, o.Zone)
		enteringActive := ev.Kind == events.MoveZone && source == ev.Obj &&
			zoneSpecContains(active, ev.To)
		if !currentlyActive && !enteringActive {
			return false
		}
	}
	return true
}

// replacementMatchesEffectCreated is the matcher for an EFFECT-created
// registration: same predicates as replacementMatchesRemembered, but the
// ActiveZones$ gate is skipped — an Effect's lifetime is active()'s, not its
// source's zone (the rule the bodyless Counter CantHappen branch already
// documents). Forge registers an Effect SA's replacements as command-zone
// entities, so their R: lines declare ActiveZones$ Command (Taii Wakeen's
// RepDamage, 18 measured DamageDone carriers) while this engine holds the
// registration in the continuous registry with the source on the battlefield
// — gating on the source's zone would permanently silence every one of them
// (task wildgrowth1). active() still ends the effect on its own lifetime.
func (e *Engine) replacementMatchesEffectCreated(r cards.Repl, source state.ObjID, ev events.Event, remembered []state.ObjID) bool {
	return e.replacementMatchesRememberedUngated(r, source, ev, remembered)
}

// replacementMatchesRememberedUngated is replacementMatchesRemembered's
// predicate body without the ActiveZones$ gate; only the two wrappers above
// reach it.
func (e *Engine) replacementMatchesRememberedUngated(r cards.Repl, source state.ObjID, ev events.Event, remembered []state.ObjID) bool {
	you := e.controllerOf(source)
	switch r.Event {
	case "Counter":
		// The Effect-created bodyless CantHappen form (Mistrise Village's
		// AntiMagic, reached only from counterReplacementMatchesAll's scan,
		// which passes a synthetic Event{Obj: target}): the remembered-scoped
		// ValidCard$ gates the countered stack object — the promise covers
		// exactly the spell the firing trigger captured. ValidSA$ is the
		// caller's counterValidSA read (shared with the printed-Repls path);
		// no ActiveZones read — an Effect's lifetime is active()'s, not its
		// source's zone.
		if v, ok := r.Params["ValidCard"]; ok {
			if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.rememberedSpecContext(you, source, remembered)) {
				return false
			}
		}
		return true
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
		// FoundSearchingLibrary$ True (Opposition Agent's "While an opponent
		// is searching their library, they exile each card they find"): the
		// replacement applies only to the moves a library search emits. The
		// host scopes that fact (BeginLibrarySearch/EndLibrarySearch around
		// effects' applyLibrarySearch); with no search in flight, or with the
		// repl's own controller the one searching, the replacement is inert.
		if raw, ok := r.Params["FoundSearchingLibrary"]; ok &&
			strings.EqualFold(strings.TrimSpace(raw), "True") {
			if e.searchingBy == 0 || e.controllerOf(source) == e.searchingBy {
				return false
			}
		}
		if v, ok := r.Params["ValidCard"]; ok {
			if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.rememberedSpecContext(you, source, remembered)) {
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
			if mo == nil || !effects.MatchesObjectCtx(e.G, v, mo, e.rememberedSpecContext(you, source, remembered)) {
				return false
			}
		}
		// CheckSVar$/SVarCompare$ (kw:etbCounter's CheckSVar$ third field --
		// Lupine Harbingers' "enters with X +1/+1 counters ... since it was
		// foretold" gate) shares replacementConditionHolds with the
		// damage/counter families. The comment above its own declaration used
		// to say the Moved case never carries these gates in the corpus; the
		// etbCounter passthrough is the one carrier, and the shared read is a
		// no-op for every Moved line without the params.
		return e.replacementConditionHolds(r, source, you)
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
		// face and applies to its own card's flip; replacementFace already
		// scanned the destination face for this event.
		if v, ok := r.Params["ValidCard"]; ok &&
			!effects.MatchesSpecFrom(e.G, v, ev.Obj, you, source) {
			return false
		}
		return e.replacementConditionHolds(r, source, you)
	case "DamageDone":
		if ev.Kind != events.Damage || !e.damageReplacementMatches(r, source, ev) {
			return false
		}
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
	case "Draw", "DrawCards":
		if ev.Kind != events.Draw {
			return false
		}
		// CR 611.3b: the ActiveZones gate above applies (a Draw replacement's
		// source is already on the battlefield — there is no entering case,
		// a card cannot replace the draw of the event that would put it into
		// play).
		if v, ok := r.Params["ValidPlayer"]; ok &&
			!effects.MatchesPlayerSpec(e.G, v, ev.Player, you) {
			return false
		}
		// The shared condition gate (CheckSVar$/IsPresent$/Hellbent$/...) —
		// every sibling case ends with it; the Draw class never read it, so
		// Quantum Riddler's LE1-over-Count$ValidHand gate (and the Hellbent
		// DrawTwo / library-empty Win carriers) fired unconditionally. No
		// repo deck carries any of the class's 39 carriers, so no golden
		// game changes (measured).
		return e.replacementConditionHolds(r, source, you)
	case "CreateToken":
		// The token-creation replacement class (Divine Visitation, Doubling
		// Season, Academy Manufactor, Xorn, ...). Applied by
		// continueCreateTokenReplacements, which reads each body's Type$
		// directly in rules — the replaceDamageAmount precedent — rather than
		// dispatching through the effects registry (no api:ReplaceToken
		// resolver exists; the census registers the name via RegisterNonAPI).
		if ev.Kind != events.TokenCreate {
			return false
		}
		// ValidToken$ names the WOULD-BE token, which does not exist yet: the
		// match is taken against a shallow read-side snapshot built off the
		// token script the event names (the same never-added-to-the-game
		// discipline StackCopy's snapshot keeps). The spec's You-side
		// predicates (YouCtrl, ...) read against the replacement SOURCE's
		// controller, while the token's controller is ev.Player — exactly how
		// Divine Visitation's "creature tokens under YOUR control" must read.
		// An unknown token key fails closed to no match.
		if v, ok := r.Params["ValidToken"]; ok {
			tok := e.tokenSnapshot(ev)
			if tok == nil || !effects.MatchesObjectCtx(e.G, v, tok,
				e.rememberedSpecContext(you, source, remembered)) {
				return false
			}
		}
		if vp, ok := r.Params["ValidPlayer"]; ok &&
			!effects.MatchesPlayerSpecFrom(e.G, vp, ev.Player, you, source) {
			return false
		}
		// EffectOnly$ True ("If an EFFECT would create ...", Doubling Season's
		// family) is READ and held: the engine's only TokenCreate emitters are
		// effect resolution (effects/token.go's effToken and effects/amass.go),
		// so today every token creation IS effect-created and the gate is
		// vacuously satisfiable. A cost-created-token provenance marker is a
		// deliberate non-goal; when one lands, this gate must read it.
		return e.replacementConditionHolds(r, source, you)
	}
	return false
}

// tokenSnapshot builds the would-be token a TokenCreate event would mint,
// as a shallow read-side object for ValidToken$ matching. Never added to
// the game — a value snapshot like StackCopy's discipline. A nil return
// (unknown token key) fails the caller's match closed.
func (e *Engine) tokenSnapshot(ev events.Event) *state.Object {
	def := e.G.Tokens[ev.Text]
	if def == nil {
		return nil
	}
	return &state.Object{Card: def, IsToken: true, Owner: ev.Player, Controller: ev.Player}
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
		var n int
		if _, hasZone := r.Params["PresentZone"]; hasZone || r.Params["PresentDefined"] != "" {
			// A damage/counter/CantPreventDamage line's IsPresent$ can name a
			// non-battlefield zone (PresentZone$) or a defined subject
			// (PresentDefined$ Self, "is this exact permanent still present");
			// countPresentInZone generalises past countPresent's fixed
			// battlefield scan for exactly those two params.
			zone := state.ZBattlefield
			if z := r.Params["PresentZone"]; z != "" {
				zone = effects.ParseZone(z)
			}
			n = e.countPresentInZone(spec, source, you, zone, r.Params["PresentDefined"])
		} else {
			n = e.countPresent(spec, source, you)
		}
		if !comparePresent(n, cmp) {
			return false
		}
	}
	// PlayerTurn$/Hellbent$/Revolt$/Delirium$/CheckDefinedPlayer$ are the
	// remaining condition gates shared by damage, counter and
	// CantPreventDamage text (the Moved/Untap/BeginPhase/Transform/
	// ProduceMana cases above never carry them in the corpus, so folding them
	// in here rather than duplicating the switch costs those cases nothing).
	if strings.EqualFold(r.Params["PlayerTurn"], "True") && e.G.Active != you {
		return false
	}
	if strings.EqualFold(r.Params["Hellbent"], "True") && len(e.G.Zone(state.ZHand, you)) != 0 {
		return false
	}
	if strings.EqualFold(r.Params["Revolt"], "True") && !e.revoltThisTurn(you) {
		return false
	}
	if strings.EqualFold(r.Params["Delirium"], "True") && e.graveyardCardTypeCount(you) < 4 {
		return false
	}
	if _, ok := r.Params["CheckDefinedPlayer"]; ok {
		// The only corpus shape is You.isMonarch. Monarch state is not yet
		// represented, so fail closed instead of preventing damage always.
		return false
	}
	if check, ok := r.Params["CheckSVar"]; ok {
		n := e.replacementCheckValue(source, check)
		if cmp := r.Params["SVarCompare"]; cmp != "" {
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

// damageReplacementMatches applies the damage-specific R: filters before the
// common active-zone gate: source and target are the actual damage source and
// recipient, and IsCombat$/DamageAmount$ describe this in-flight event.
func (e *Engine) damageReplacementMatches(r cards.Repl, source state.ObjID, ev events.Event) bool {
	ctrl := e.controllerOf(source)
	if v := r.Params["ValidCause"]; v != "" && !e.replacementCauseMatches(v, source, e.damaging) {
		return false
	}
	// A DB$ ReplaceDamage body must RESOLVE its Amount$ before this
	// replacement may match: the body is Forge's "prevent N of that damage"
	// idiom, and a match this build cannot price would previously be applied
	// as a silent FULL prevention (the body's emissions were supposed to
	// replace the event, so the engine discarded it -- and the body emitted
	// nothing). Amount$ values resolvable through the shared numeric grammar
	// (a literal; an SVar such as Battletide's AlchemicX or a card-defined
	// ShieldAmount; an inline Count$/ReplaceCount$ expression) match; an
	// unmodelled value frame (Power Leak's PaidAmount, an undefined name)
	// fails closed and leaves the damage untouched, per CR 616.1's "only
	// applicable replacements apply".
	if r.With != nil && r.With.API == "ReplaceDamage" {
		if _, ok := e.replaceDamageAmount(ev, replMatch{id: source, repl: &r}); !ok {
			return false
		}
	}
	// An Optional$ True damage replacement whose OptionalDecider$ names a
	// frame this build does not resolve also fails closed: asking the damaged
	// player would answer a "may" that belongs to somebody else (the
	// Battletide Alchemist round-2 finding). An ABSENT parameter keeps the
	// historical default, where the affected player answers.
	if strings.EqualFold(r.Params["Optional"], "True") {
		if v := strings.TrimSpace(r.Params["OptionalDecider"]); v != "" && v != "You" {
			return false
		}
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

func (e *Engine) replacementCheckValue(source state.ObjID, check string) int32 {
	o := e.G.Obj(source)
	if o == nil {
		return 0
	}
	ctx := e.replCtx(replMatch{id: source}, events.Event{})
	// The face's own SVar table comes FIRST: a CheckSVar$ X gate whose face
	// defines a real SVar:X body (Steel Exemplar's Count$Converge, Walking
	// Dream's PlayerCountOpponents$ head, the multiclass_baldric and
	// spirit_of_resistance bodies the switch below implements) must evaluate
	// THAT body through the machinery below -- the announced-X shortcut is
	// only the fallback for a face that defines no X. For a body that READS
	// the announced X (banefire's Count$xPaid) the two coincide, so the
	// reorder changes nothing for it.
	body := check
	if ctx.SVars != nil {
		if v, ok := ctx.SVars[check]; ok {
			body = v
		}
	}
	if body == check && check == "X" {
		return o.X
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
			colors += e.objColors(e.G.Obj(id))
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
			kind: replChoiceCounter,
			ev:   events.Event{Obj: target}, cands: matches, before: e.triggerBefore,
			player: t.Controller, cause: cause,
		})
		if e.pending == nil {
			e.askReplacementChoice(t.Controller)
		}
		return false
	}
}

func (e *Engine) counterReplacementMatchesAll(target, cause state.ObjID) []replMatch {
	var matches []replMatch
	// Effect-created Counter replacements (Mistrise Village's AntiMagic:
	// "the next spell you cast this turn can't be countered", a delayed
	// Effect whose body is a bodyless Layer$ CantHappen R:): the continuous
	// registry is the only place these live, so the Counter path — whose
	// ordinary scan reads printed face Repls — matches them here through the
	// same remembered-scoped matcher the general replacement scan uses, plus
	// the shared ValidSA$ subset gate. Stopping the Counter event (the
	// With-less form) is the complete replacement.
	for _, ce := range e.active() {
		if ce.ReplacementEvent != "Counter" || ce.ReplacementBody != "" ||
			!strings.EqualFold(strings.TrimSpace(ce.ReplacementParams["Layer"]), "CantHappen") {
			continue
		}
		r := cards.Repl{Event: "Counter", Params: ce.ReplacementParams}
		if !e.replacementMatchesEffectCreated(r, ce.Source, events.Event{Obj: target}, ce.Remembered) {
			continue
		}
		t := e.G.Obj(target)
		src := e.G.Obj(ce.Source)
		if t == nil || src == nil {
			continue
		}
		if spec := r.Params["ValidSA"]; spec != "" && !counterValidSA(e.G, t, spec, e.controllerOf(ce.Source), ce.Source) {
			continue
		}
		matches = append(matches, replMatch{id: ce.Source, repl: &r,
			remembered: ce.Remembered, chosen: ce.ChosenNumber,
			key: "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))})
	}
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
	if !e.replacementConditionHolds(r, source, o.Controller) {
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
	// replChoiceDamage parks a CR 616.1 order competition for an object
	// Damage event (repl:DamageDone): two or more applicable damage
	// replacements (a Prevent$ True shield, a DB$ ReplaceDamage subtraction,
	// a DB$ ReplaceEffect amount rewrite) and the affected player orders
	// them. Recomputed every cycle (CR 616.1e) rather than driven by
	// applied/applicable like the mana/phase kinds, because a redirect or
	// amount rewrite can change which of the ORIGINAL candidates still apply.
	replChoiceDamage
	// replChoiceCounter parks CR 616.1's order competition when two or more
	// repl:Counter replacements would stop the same spell/ability.
	replChoiceCounter
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
	// life marks a life gain/loss competition (applyLifeReplacements): ev is
	// then the event as already modified by appliedRepls, the affected player
	// is ev.Player, and the answer continues the CR 616.1 loop rather than
	// finishing after one application. damaging, combatDamaging and
	// dmgSrcOverride are the synchronous damage context the parked event was
	// proposed under, restored while the answer emits it so provenance-reading
	// triggers and protection see the same source.
	life           bool
	appliedRepls   []replMatch
	damaging       state.ObjID
	combatDamaging bool
	dmgSrcOverride state.ObjID
	// used is kind == replChoiceDamage's own applied-set: the matches already
	// settled this CR 616.1e recomputation cycle for an object Damage event
	// (repl:DamageDone), tracked by value rather than applied's per-candidate
	// bool index because remainingDamageReplacements recomputes the candidate
	// set itself after every rewrite instead of indexing a fixed cands slice.
	// player is the resolved asking player (replacementAskPlayer's result,
	// which may differ from the CR 616.1e affected player recomputed fresh
	// into damageAffectedPlayer at every cycle) for kind == replChoiceDamage
	// or replChoiceCounter.
	used   []replMatch
	player state.PlayerID
	// combat marks a damage competition parked from the combat-damage step's
	// own assignment loop (rules/combat.go): the chosen replacement's
	// lifelink/deathtouch riders and commander-damage tally pay the way
	// ordinary combat damage does, and finishChosenDamage resumes
	// runCombatAssignments once this batch's parked choices all settle.
	// lifelink/deadly cache the damage source's keywords at park time (Task
	// 15's provenance-freezing discipline), and cause is the Counter
	// replacement's cause object (counterReplacementMatches' third argument),
	// set only when kind == replChoiceCounter.
	combat   bool
	lifelink bool
	deadly   bool
	cause    state.ObjID
}

// replacementChoicePlayer is the affected player a parked competition asks:
// a life event's player (the player whose life total changes), else mana/
// phase candidates by role, else the moving object's controller.
func (e *Engine) replacementChoicePlayer(rc replChoice) (state.PlayerID, bool) {
	if rc.life {
		return rc.ev.Player, int(rc.ev.Player) < len(e.G.Players)
	}
	switch rc.kind {
	case replChoiceMana, replChoiceManaColor:
		return rc.ev.Player, int(rc.ev.Player) < len(e.G.Players)
	case replChoicePhaseOrder, replChoicePhaseOptional:
		return e.G.Active, int(e.G.Active) < len(e.G.Players)
	case replChoiceDamage, replChoiceCounter:
		// Already resolved by poseDamageReplacementChoice/the recomputation
		// cycle (replacementAskPlayer over the freshly recomputed CR 616.1e
		// affected player), never re-derived from rc.ev.Obj's controller here
		// -- a redirect or an OptionalDecider$ can make the asking player
		// differ from that.
		return rc.player, int(rc.player) < len(e.G.Players)
	default:
		// A replaced DRAW event has no object whose controller could be
		// consulted -- the affected player is the draw-er itself (the same
		// binding the replacement's ReplacedPlayer context carries).
		if rc.ev.Kind == events.Draw {
			return rc.ev.Player, int(rc.ev.Player) < len(e.G.Players)
		}
		o := e.G.Obj(rc.ev.Obj)
		if o == nil || int(o.Controller) >= len(e.G.Players) {
			return 0, false
		}
		return o.Controller, true
	}
}

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

// poseReplacementChoice starts a CR 616.1 order-selection suspension: the
// competing event is parked and the affected controller -- the controller of
// the moving object, CR 616.1's "affected player" -- is asked which
// replacement applies first. Mirrors commander-zone parking: the ask is posed
// only when no other decision is already pending (the caller has already
// ruled out a departed controller, which makes no choices under CR 800.4a).
func (e *Engine) poseReplacementChoice(ev events.Event, matches []replMatch) {
	p := ev.Player
	if ev.Kind != events.Draw {
		o := e.G.Obj(ev.Obj)
		if o == nil {
			return
		}
		p = o.Controller
	}
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
func (e *Engine) poseDamageReplacementChoice(ev events.Event, matches []replMatch, p state.PlayerID) {
	source := e.protectionSource(e.damaging)
	e.replChoices = append(e.replChoices, replChoice{
		kind: replChoiceDamage, ev: ev, cands: matches, before: e.triggerBefore, player: p,
		damaging: source, combat: e.combatDamaging,
		lifelink: e.HasKeyword(source, "Lifelink"), deadly: e.HasKeyword(source, "Deathtouch"),
	})
	if e.pending == nil {
		e.askReplacementChoice(p)
	}
}

func (e *Engine) askReplacementChoice(p state.PlayerID) {
	rc := e.replChoices[0]
	d := &decision.Decision{Player: p, Kind: decision.KReplacement, Min: 1, Max: 1,
		Source: rc.ev.Obj, ResumeKind: "replacement"}
	indices := make([]int, len(rc.cands))
	for i := range indices {
		indices[i] = i
	}
	switch rc.kind {
	case replChoiceDamage:
		d.Prompt = "Several replacement effects would modify damage: choose which applies next."
	case replChoiceCounter:
		d.Prompt = "Several replacement effects would modify this counter event: choose which applies."
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
		prompt := "Several replacement effects would change how " + name + " moves: choose which applies first."
		if rc.life {
			what := "life gain"
			if _, _, loss := lifeLoss(rc.ev); loss {
				what = "life loss"
			}
			prompt = "Several replacement effects would change this " + what + ": choose the one that applies first."
		}
		d.Prompt = prompt
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
	if rc.ev.Kind == events.Damage && hasOptionalReplacement(rc.cands) {
		d.Options = append(d.Options, decision.Option{Index: len(rc.cands), Kind: "skip_replacement",
			Label: "Do not apply an optional replacement"})
		// A single-optional competition asked of its OptionalDecider$ is the
		// replacement's own "may", not an order among several: pose it as
		// one (Battletide Alchemist round-2 finding).
		if len(rc.cands) == 1 && strings.EqualFold(rc.cands[0].repl.Params["Optional"], "True") {
			label := "Apply the replacement"
			if o := e.G.Obj(rc.cands[0].id); o != nil && o.Face() != nil && o.Face().Name != "" {
				label = "Apply " + o.Face().Name + "'s replacement"
			}
			d.Prompt = label + " to this damage?"
		}
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
	rp := e.resume
	chosen := d.Chosen(in)
	damageKind := rc.kind == replChoiceDamage || rc.kind == replChoiceCounter
	if len(chosen) == 0 || (damageKind && (chosen[0].Index < 0 || chosen[0].Index > len(rc.cands) ||
		(chosen[0].Index == len(rc.cands) && !(rc.kind == replChoiceDamage && hasOptionalReplacement(rc.cands))))) {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "replacement answer had no choice"})
		return
	}
	before := e.triggerBefore
	e.triggerBefore = rc.before
	if damageKind {
		completed := true
		switch rc.kind {
		case replChoiceCounter:
			e.applyChosenCounterReplacement(rc, chosen[0].Index)
		case replChoiceDamage:
			completed = e.handleDamageReplacementChoice(rc, chosen[0].Index)
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
				// while parked asks the NEW affected player -- except that a
				// single-optional competition's "may" still belongs to its
				// OptionalDecider$ (replacementAskPlayer).
				if p, ok := e.damageAffectedPlayer(next.ev); ok && !e.G.Players[p].Lost {
					e.replChoices[0].player = e.replacementAskPlayer(next.cands, p)
					if e.pending == nil {
						e.askReplacementChoice(e.replChoices[0].player)
					}
					return
				}
				// CR 800.4a: an affected player who has left the game or lost
				// makes no choices. Its candidates apply in deterministic scan
				// order and the queue drains on.
				e.replChoices = e.replChoices[1:]
				switch {
				case next.kind == replChoiceCounter:
					e.applyChosenCounterReplacement(next, 0)
				case next.kind == replChoiceDamage:
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
			// remaining precomputed assignments before running SBAs or the
			// regular pass; a newly parked replacement simply returns again.
			e.damageStep(false)
			if e.pending == nil && e.combatRound.assignments == nil && e.combatRound.active {
				e.completeCombatPass(e.combatRound.pass)
			}
		}
		if completed && rp != nil && e.resume == rp {
			// The parked event and its riders are complete. Resume only the
			// chain after the effect that proposed it; the effect itself must
			// not emit the same damage/counter event a second time.
			e.resume = nil
			e.resumeResolution(rp, nil)
		}
		return
	}
	if rc.life {
		// Apply the chosen replacement, then re-evaluate what still applies
		// to the modified event (CR 616.1e); a further non-commuting
		// competition asks again at the front of the queue.
		if chosen[0].Index >= len(rc.cands) {
			e.triggerBefore = before
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "life replacement answer out of range"})
			return
		}
		damaging, combat, override := e.damaging, e.combatDamaging, e.dmgSrcOverride
		e.damaging, e.combatDamaging, e.dmgSrcOverride = rc.damaging, rc.combatDamaging, rc.dmgSrcOverride
		m := rc.cands[chosen[0].Index]
		if next, consumed := e.applyLifeReplacement(rc.ev, m); !consumed {
			applied := append(append([]replMatch(nil), rc.appliedRepls...), m)
			e.continueLifeReplacements(next, applied)
		}
		e.damaging, e.combatDamaging, e.dmgSrcOverride = damaging, combat, override
		e.triggerBefore = before
		e.askNextReplacementChoice()
		return
	}
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
		if ce.ReplacementEvent == "" {
			continue
		}
		if with := replacementBodySA(ce.ReplacementBody); with != nil {
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams, With: with}
			m := replMatch{id: ce.Source, repl: r,
				remembered: ce.Remembered, chosen: ce.ChosenNumber,
				key: "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))}
			if !alreadyUsed(m) && e.replacementMatchesEffectCreated(*r, ce.Source, ev, ce.Remembered) &&
				!(damageReplacementPrevents(*r) && e.cantPreventDamage(e.damaging, ev.Obj)) {
				out = append(out, m)
			}
		} else if ce.ReplacementBody == "" && ce.ReplacementEvent == "DamageDone" &&
			strings.EqualFold(ce.ReplacementParams["Prevent"], "True") {
			// Mirror applyReplacementsDispatch's bodyless-Prevent admission
			// (dponce1 r2): after a first NONTERMINAL application (a partial
			// DB$ ReplaceDamage body reduced the held event), the recomputed
			// CR 616.1e candidate set must still hold the bodyless "prevent
			// all" effect — otherwise the remaining damage the registration
			// exists to prevent lands silently.
			r := &cards.Repl{Event: ce.ReplacementEvent, Params: ce.ReplacementParams}
			m := replMatch{id: ce.Source, repl: r,
				remembered: ce.Remembered,
				key:        "effect:" + strconv.Itoa(int(ce.Source)) + ":" + strconv.Itoa(int(ce.Timestamp))}
			if !alreadyUsed(m) && e.replacementMatchesEffectCreated(*r, ce.Source, ev, ce.Remembered) &&
				!(damageReplacementPrevents(*r) && e.cantPreventDamage(e.damaging, ev.Obj)) {
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
			if damageReplacementPrevents(*r) && e.cantPreventDamage(e.damaging, ev.Obj) {
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
	// Defense in depth: both candidate-producing paths already filter
	// prevention bodies out under CantPreventDamage, but the application
	// point re-checks so a future selection path cannot reintroduce the
	// leak. A skipped body is nonterminal, so the caller recomputes the
	// remaining candidates against the still-standing event (m is in
	// rc.used, so it cannot be picked twice).
	if damageReplacementPrevents(*m.repl) && e.cantPreventDamage(e.damaging, ev.Obj) {
		return false
	}
	if strings.EqualFold(m.repl.Params["Prevent"], "True") {
		// The ordered path's full prevention is terminal — the held event
		// never lands — so this re-entrant Note is the prevention's only log
		// record, the same shape applyNonMoveReplacements' Prevent$ arm
		// stores (dponce1 r2: silently returning terminal recorded nothing,
		// so no DamagePreventedOnce trigger could fire off a chosen
		// prevention). Amount is the held event's REMAINING amount: a
		// partial DB$ ReplaceDamage body may already have reduced it (CR
		// 616.1e), and TriggerCount$DamageAmount reads the amount THIS
		// prevention prevented.
		e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
			Amount: ev.Amount, Text: "damage prevented by replacement effect"})
		return true
	}
	if m.repl.With == nil {
		return true
	}
	if m.repl.With.API == "ReplaceDamage" {
		// The body subtracts its Amount from the held event; a fully
		// prevented event is terminal, a reduced one stands for the
		// recomputation below (CR 616.1e).
		return e.applyReplaceDamageBody(ev, m)
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

// applyLifeReplacements evaluates the GainLife and LifeReduced replacements
// that apply to a proposed life gain or life loss before it reaches the log.
// The transformations are read from the replacement body's ReplaceCount$
// grammar, not card names, so Archives, Reflection, Cleric Class, Bloodletter
// and their corpus siblings share one path.
//
// CR 616.1: when more than one replacement would modify the event, the
// affected player chooses one to apply, then applicability is re-checked
// against the modified event (CR 616.1e) and the choice repeats until none is
// left. Each replacement applies at most once to the event (CR 614.5). The
// order choice reuses the engine's one KReplacement decision path
// (poseLifeReplacementChoice / handleReplacement); it is skipped only when
// every competing replacement is the same commuting operator (all doublers,
// all "plus N", all prevention), where every order produces the same event.
func (e *Engine) applyLifeReplacements(ev events.Event) (events.Event, bool) {
	if ev.Kind == events.LifeChange && ev.Amount > 0 && e.lifeGainForbidden(ev.Player) {
		return e.emit(events.Event{Kind: events.Note, Player: ev.Player, Text: "prevented: cannot gain life"}), true
	}
	return e.continueLifeReplacements(ev, nil)
}

// continueLifeReplacements applies the remaining applicable replacements to
// ev, which the replacements in applied have already modified. It returns
// handled=true whenever anything replaced the event or a choice was parked.
func (e *Engine) continueLifeReplacements(ev events.Event, applied []replMatch) (events.Event, bool) {
	for {
		cands := e.lifeReplacementCandidates(ev, applied)
		if len(cands) == 0 {
			if len(applied) == 0 {
				return ev, false
			}
			return e.emitLifeReplacement(ev)
		}
		if len(cands) > 1 && !e.lifeReplacementsCommute(ev, cands) && e.poseLifeReplacementChoice(ev, cands, applied) {
			return ev, true
		}
		m := cands[0]
		next, consumed := e.applyLifeReplacement(ev, m)
		if consumed {
			return ev, true
		}
		ev = next
		applied = append(applied[:len(applied):len(applied)], m)
	}
}

// poseLifeReplacementChoice parks a life event whose competing replacements
// do not commute and asks the affected player (CR 616.1: the player whose life
// total the event changes) which applies first. It declines, and the caller
// applies the first candidate in deterministic scan order, only where no
// choice can be made: the player has left the game (CR 800.4a) or another
// decision is already outstanding, which a second ask would overwrite.
func (e *Engine) poseLifeReplacementChoice(ev events.Event, cands, applied []replMatch) bool {
	p := ev.Player
	if int(p) >= len(e.G.Players) || e.G.Players[p].Lost || e.pending != nil {
		return false
	}
	rc := replChoice{ev: ev, cands: cands, before: e.triggerBefore, life: true,
		appliedRepls: applied, damaging: e.damaging, combatDamaging: e.combatDamaging,
		dmgSrcOverride: e.dmgSrcOverride}
	// The front of the queue is the competition being asked. A life choice is
	// asked immediately (pending is nil), so it goes first.
	e.replChoices = append([]replChoice{rc}, e.replChoices...)
	e.askReplacementChoice(p)
	return true
}

// lifeReplacementCandidates collects, in forEachObject's deterministic scan
// order, every replacement not yet applied to ev that would modify it now.
// A gain is only ever modified by GainLife replacements and a loss only by
// LifeReduced ones, so a replacement that turns a gain into a loss (Tainted
// Remedy) leaves every other GainLife replacement inapplicable.
func (e *Engine) lifeReplacementCandidates(ev events.Event, applied []replMatch) []replMatch {
	event, p, loss := "", state.PlayerID(0), int32(0)
	if ev.Kind == events.LifeChange && ev.Amount > 0 {
		event, p = "GainLife", ev.Player
	} else if q, amount, ok := lifeLoss(ev); ok {
		event, p, loss = "LifeReduced", q, amount
	} else {
		return nil
	}
	var out []replMatch
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			return
		}
		for i := range o.Face().Repls {
			r := &o.Face().Repls[i]
			if r.Event != event || !replacementActive(e, id, r) || !replacementPlayerMatches(e, id, r, p) ||
				lifeReplacementApplied(applied, id, r) {
				continue
			}
			if e.lifeReplacementApplies(ev, id, r, p, loss) {
				out = append(out, replMatch{id: id, repl: r})
			}
		}
	})
	return out
}

func lifeReplacementApplied(applied []replMatch, id state.ObjID, r *cards.Repl) bool {
	for _, m := range applied {
		if m.id == id && m.repl == r {
			return true
		}
	}
	return false
}

// lifeReplacementApplies reports whether replacement r of source would do
// something to ev. A replacement whose body this engine cannot perform is not
// a candidate, so it never occupies an order choice.
func (e *Engine) lifeReplacementApplies(ev events.Event, source state.ObjID, r *cards.Repl, p state.PlayerID, loss int32) bool {
	if r.Event == "GainLife" {
		if strings.EqualFold(r.Params["Prevent"], "True") {
			return true
		}
		if !e.replacementCondition(source, r) || r.Params["ValidSource"] != "" {
			return false
		}
		if _, ok := e.replaceCount(source, r, "LifeGained", ev.Amount); ok {
			return true
		}
		return r.With != nil && (r.With.API == "LoseLife" || r.With.API == "Draw")
	}
	if strings.EqualFold(r.Params["IsDamage"], "True") && ev.Kind != events.Damage {
		return false
	}
	if strings.EqualFold(r.Params["PlayerTurn"], "True") && e.G.Active != e.controllerOf(source) {
		return false
	}
	if !e.replacementCondition(source, r) {
		return false
	}
	if result := r.Params["Result"]; result != "" && !compareLife(e.G.Players[p].Life-loss, result) {
		return false
	}
	if _, ok := e.replaceCount(source, r, "Amount", loss); ok {
		return true
	}
	// A non-ReplaceEffect body (Enduring Angel's transform then SetLife)
	// wholly replaces the loss.
	o := e.G.Obj(source)
	return r.With != nil && o != nil && o.Face() != nil
}

// lifeReplacementsCommute reports whether every order of cands yields the same
// event: all prevent the gain, all double, or all add a constant, and none
// gates its own applicability on the running amount (Result$).
func (e *Engine) lifeReplacementsCommute(ev events.Event, cands []replMatch) bool {
	name := "Amount"
	if ev.Kind == events.LifeChange && ev.Amount > 0 {
		name = "LifeGained"
	}
	kind := ""
	for _, m := range cands {
		if m.repl.Params["Result"] != "" {
			return false
		}
		k := ""
		switch op, ok := e.replaceCountOp(m.id, m.repl, name); {
		case m.repl.Event == "GainLife" && strings.EqualFold(m.repl.Params["Prevent"], "True"):
			k = "prevent"
		case ok && op == "/Twice":
			k = "twice"
		case ok && strings.HasPrefix(op, "/Plus."):
			k = "plus"
		default:
			return false
		}
		if kind != "" && k != kind {
			return false
		}
		kind = k
	}
	return true
}

// applyLifeReplacement applies one chosen replacement to ev. It returns the
// modified event, or consumed=true when the replacement's own body wholly
// replaced the event (prevention, a draw or a primitive chain instead).
func (e *Engine) applyLifeReplacement(ev events.Event, m replMatch) (events.Event, bool) {
	r := m.repl
	if r.Event == "GainLife" {
		if strings.EqualFold(r.Params["Prevent"], "True") {
			e.emit(events.Event{Kind: events.Note, Player: ev.Player, Text: "prevented: cannot gain life"})
			return ev, true
		}
		if amount, ok := e.replaceCount(m.id, r, "LifeGained", ev.Amount); ok {
			ev.Amount = amount
			return ev, false
		}
		switch r.With.API {
		case "LoseLife":
			// That player loses that much life instead: the event is now a
			// loss, so only LifeReduced replacements can modify it further.
			ev.Amount = -ev.Amount
			return ev, false
		case "Draw":
			e.lifeReplacementDraw(ev.Player, ev.Amount)
			return ev, true
		}
		return ev, false
	}
	_, loss, _ := lifeLoss(ev)
	if amount, ok := e.replaceCount(m.id, r, "Amount", loss); ok {
		if ev.Kind == events.Damage {
			ev.Amount = amount
		} else {
			ev.Amount = -amount
		}
		return ev, false
	}
	o := e.G.Obj(m.id)
	if o == nil || o.Face() == nil || r.With == nil {
		// The source no longer exists (a parked choice answered after it
		// left): its replacement has nothing left to perform.
		return ev, false
	}
	e.runReplaceWith(&effects.Ctx{Source: m.id, Controller: o.Controller, SVars: o.Face().SVars}, 0, r.With, nil)
	return ev, true
}

// lifeReplacementDraw draws n cards for a GainLife→Draw replacement body
// (Lich's "If you would gain life, draw that many cards instead"),
// suspension-aware: each DrawFor may pose a Dredge ask (CR 702.55) and
// suspend. The loop parks the remaining count on the ask's resume point
// (resolution.go) and returns, instead of looping on -- looping on would
// pose a SECOND ask while the first is outstanding, orphaning it and losing
// the remaining draws (findings-sol4 MAJOR). The answered dredge re-drives
// the rest from handleModes' direct arm (stack empty) or resumeResolution's
// dredge arm (a resolving object on the stack), both of which drain any
// replacement-order queue the interrupted pass left behind.
func (e *Engine) lifeReplacementDraw(p state.PlayerID, n int32) {
	for i := int32(0); i < n; i++ {
		effects.DrawFor(e, p)
		if e.Suspended() {
			e.resume.lifeDraws = n - (i + 1)
			return
		}
	}
}

// emitLifeReplacement logs a fully transformed event without starting a new
// replacement pass: every applicable replacement has had its one opportunity.
// A gain reduced to nothing (LimitMax of zero) is no event at all.
func (e *Engine) emitLifeReplacement(ev events.Event) (events.Event, bool) {
	if ev.Kind == events.LifeChange && ev.Amount == 0 {
		return ev, true
	}
	saved := e.applyingReplacement
	e.applyingReplacement = true
	stored := e.emit(ev)
	e.applyingReplacement = saved
	return stored, true
}

// replacementCondition reads the common CheckSVar$/SVarCompare$ gate (Phial
// of Galadriel) from the replacement source's current context.
func (e *Engine) replacementCondition(source state.ObjID, r *cards.Repl) bool {
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return false
	}
	if spec := r.Params["IsPresent"]; spec != "" {
		found := false
		for _, p := range e.G.AliveFrom(0) {
			for _, id := range e.G.Zone(state.ZBattlefield, p) {
				if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, o.Controller)) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	name := r.Params["CheckSVar"]
	if name == "" {
		return true
	}
	value := effects.EvalCount(e, &effects.Ctx{Source: source, Controller: o.Controller, SVars: o.Face().SVars}, o.Face().SVars[name])
	return compareLife(value, r.Params["SVarCompare"])
}

// replaceCount evaluates ReplaceEffect's ReplaceCount$Amount/LifeGained
// forms. It follows SVar indirection and supports the three corpus operators:
// Twice, Plus.N and LimitMax.<SVar>.
func (e *Engine) replaceCount(source state.ObjID, r *cards.Repl, name string, amount int32) (int32, bool) {
	op, ok := e.replaceCountOp(source, r, name)
	if !ok {
		return 0, false
	}
	o := e.G.Obj(source)
	if op == "/Twice" {
		return amount * 2, true
	}
	if n, err := strconv.ParseInt(strings.TrimPrefix(op, "/Plus."), 10, 32); strings.HasPrefix(op, "/Plus.") && err == nil {
		return amount + int32(n), true
	}
	if arg, ok := strings.CutPrefix(op, "/LimitMax."); ok {
		limit := effects.EvalCount(e, &effects.Ctx{Source: source, Controller: o.Controller, SVars: o.Face().SVars}, o.Face().SVars[arg])
		if limit < 0 {
			limit = 0
		}
		if amount > limit {
			amount = limit
		}
		return amount, true
	}
	return 0, false
}

// replaceCountOp returns the operator suffix ("/Twice", "/Plus.1", ...) of a
// ReplaceEffect body's ReplaceCount$<name> value, following SVar indirection.
func (e *Engine) replaceCountOp(source state.ObjID, r *cards.Repl, name string) (string, bool) {
	if r.With == nil || r.With.API != "ReplaceEffect" || !strings.EqualFold(r.With.Params["VarName"], name) {
		return "", false
	}
	expr := r.With.Params["VarValue"]
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil {
		return "", false
	}
	if body, ok := o.Face().SVars[expr]; ok {
		expr = body
	}
	prefix := "ReplaceCount$" + name
	if !strings.HasPrefix(expr, prefix) {
		return "", false
	}
	return strings.TrimPrefix(expr, prefix), true
}

// lifeGainForbidden checks active CantGainLife statics against the player who
// would gain life. R:Event$ GainLife Prevent$ True lines are replacement
// effects and compete in the CR 616.1 order choice instead. The static's
// ValidPlayer$ scope is read here, in the static's own parameter bucket.
func (e *Engine) lifeGainForbidden(p state.PlayerID) bool {
	for _, sv := range e.activeStatics("CantGainLife") {
		if spec := sv.Params["ValidPlayer"]; spec == "" ||
			effects.MatchesPlayerSpec(e.G, spec, p, sv.Controller) {
			return true
		}
	}
	return false
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
	// applyLifeReplacements. repl:DamageDone and repl:Counter are this
	// ticket's own additions, matched by replacementMatches's DamageDone case
	// and CounterAllowed respectively.
	effects.RegisterNonAPI("kw:etbCounter", "kw:ETBReplacement",
		"repl:Untap", "repl:BeginPhase", "repl:Transform", "repl:ProduceMana",
		"repl:GainLife", "repl:LifeReduced", "repl:DamageDone", "repl:Counter",
		"repl:CreateToken", "api:ReplaceToken")
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
