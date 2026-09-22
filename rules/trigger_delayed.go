// Delayed triggers: the ones armed now to fire later.
//
// Both the turn-scoped delayed triggers and the event-scoped ones, which carry
// their own cast matcher because a delayed SpellCast is scoped to the delay, not
// to the face.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// checkTriggers is called from emit after every event. It walks every
// object once (forEachObject) and, for each cards.Trigger on that object's
// face, asks triggerMatches whether this event satisfies it. A match
// appends to e.pendingTriggers with a context whose Remembered holds the
// triggering object; putTriggersOnStack later drains that queue onto the
// stack in APNAP order.
//
// lki is the object a MoveZone/Draw/PutOnStack event's own Obj was, just
// before emit applied the event (nil for every other event kind, or if that
// object could not be found -- see emit). It describes ev.Obj, not id (the
// object whose T: line is being checked in a given iteration below), so it
// is handed to EVERY trigger this event fires, not only a Card.Self trigger
// on ev.Obj itself: a bystander's "whenever a creature you control dies"
// wants the LKI of the creature that died, exactly the same as that
// creature's own dies trigger would. Fix round 1, Important 2: an earlier
// version of this doc claimed LKI was withheld from every trigger but the
// matching object's own, which was never what the code below does (see
// objLKI's guard) -- lki.ID == ev.Obj is already guaranteed by construction
// in emit (lki, when non-nil, is always built from e.G.Obj(ev.Obj)), so
// there is no per-trigger gate here at all, only a defensive belt-and-
// braces check against a future emit change that might one day pass a
// mismatched lki.
// checkDelayedTriggers queues a pending trigger for every delayed-trigger
// registration (state.Game.Delayed) registered to fire on the step just
// entered. It runs from checkTriggers after a StepChange event, alongside
// the ordinary face-trigger walk, so a delayed trigger reaches the stack
// through exactly the same putTriggersOnStack drain as any T: line trigger.
// The registration carries everything the drain needs: the source permanent
// (whose SVar table holds the Execute$ sub-ability), the controller, the
// Execute$ SVar name and the Remembered captured at registration.
//
// CR 603.7: a delayed trigger fires even when its source has left the
// battlefield (the registration is independent of it once created), so the
// source is read only to resolve the Execute$ SVar table -- an object that
// has moved zones (in a graveyard, exiled) still has a Face and SVars, so
// the trigger still fires; a source that has ceased to exist entirely (a
// token or copy gone from the board) has nothing to read and degrades to a
// no-op. The one-shot removal happens in events.Apply's DelayedPush case, so
// the drain never re-fires the same registration on a later occurrence of
// the phase.
func (e *Engine) checkDelayedTriggers(ev events.Event) {
	for i := range e.G.Delayed {
		dt := &e.G.Delayed[i]
		if dt.EventMode != "" {
			continue // an event-matched registration fires on its event, never a step
		}
		if dt.Phase != ev.Step {
			continue
		}
		// CR 603.7 + CR 500.7 (rules/saga.go, events.ExtraTurn): a delayed
		// trigger an extra-turn grant registered carries the granted turn's
		// number as MinTurn, so the granting turn's own occurrence of the
		// phase (Final Fortune's end step) does not consume the one-shot
		// registration -- the trigger fires exactly once, in the granted
		// turn, and the registration stays pending until then.
		if dt.MinTurn > 0 && e.G.Turn < dt.MinTurn {
			continue
		}
		// The MaxTurn mirror (ThisTurn$ True, Mistrise Village): a
		// registration whose expiry turn has passed never fires. The entry
		// is skipped, not removed -- removal would need its own event for a
		// replay to fold, and an expired one-shot is inert either way.
		if dt.MaxTurn > 0 && e.G.Turn > dt.MaxTurn {
			continue
		}
		// ValidPlayer$ (Necropotence's "at the beginning of YOUR next end
		// step"): the registering DelayedTrigger SA's ValidPlayer$ filter,
		// carried on the registration and evaluated at the phase occurrence
		// the way phaseMatches evaluates a Mode$ Phase T: line's own
		// ValidPlayer$ -- the step just entered always belongs to the current
		// active player, so the gate asks MatchesPlayerSpec about e.G.Active
		// against the REGISTRATION's controller. A gate the step fails leaves
		// the one-shot registration pending (the DelayedPush that would
		// consume it never mints), so it fires at the first later occurrence
		// of the phase that does match. The corpus's DelayedTrigger
		// ValidPlayer$ values are Player 135, You 35, Opponent 2 plus a
		// handful of qualified forms; bare Player matches every seat (the
		// ungated behaviour those registrations already had), and the
		// qualified ones fail closed inside MatchesPlayerSpec (the fx20
		// convention: an unmodellable qualifier fires for nobody, never for
		// everybody).
		if dt.ValidPlayer != "" && !effects.MatchesPlayerSpec(e.G, dt.ValidPlayer, e.G.Active, dt.Controller) {
			continue
		}
		if int(dt.Controller) >= len(e.G.Players) || e.G.Players[dt.Controller].Lost {
			continue
		}
		src := e.G.Obj(dt.Source)
		if src == nil {
			continue
		}
		f := src.Face()
		if f == nil {
			continue
		}
		sa := cards.ResolveSVar(f.SVars, dt.Execute)
		if sa == nil {
			continue
		}
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     dt.Source,
			Controller: dt.Controller,
			Delayed:    true,
			DelayedID:  dt.ID,
			Execute:    dt.Execute,
			SA:         sa,
			Ctx: effects.Ctx{
				Source:     dt.Source,
				Controller: dt.Controller,
				Remembered: append([]state.Target(nil), dt.Remembered...),
				Captured:   append([]state.Target(nil), dt.Remembered...),
			},
		})
	}
}

// checkEventDelayedTriggers queues a pending trigger for every event-matched
// delayed registration (state.Game.Delayed entries with EventMode set) that
// the event just folded satisfies. A Mode$ Phase registration fires on the
// step it names (checkDelayedTriggers, above); the opening-hand Effect shape
// (Chancellor of the Annex) registers Mode$ SpellCast one-off triggers, which
// fire on a spell's PutOnStack exactly like a face SpellCast trigger. The
// registration's STORED trigger body (dt.Trigger) is re-parsed at fire time
// so its validity clauses are evaluated against the actual cast -- the
// registration is the game state a replay rebuilds, the body is not.
//
// "You" inside that body is the REGISTRATION's controller -- the effect owner
// the registration was minted for, not the source card's controller: the
// Chancellor's Effect is EffectOwner$ Opponent, so each per-opponent
// registration fires on that opponent's first cast, which is the oracle's
// "when each opponent casts their first spell". TriggerZones$ is deliberately
// not consulted: Forge parks the trigger on a command-zone Effect, while the
// engine's registration itself is that presence -- the source Chancellor card
// stays in its hand.
// delayedSpellCastFire is one event-matched SpellCast registration the scan
// pass of checkEventDelayedTriggers collected, with everything its firing
// pass needs. The scan is deliberately PURE — it must not emit — because the
// static firing arm's synchronous DelayedPush consumes its registration via
// events.Apply, which splices state.Game.Delayed mid-iteration: firing inside
// the live-slice range skipped or crashed on the registrations after the
// first matched one (two "the next spell you cast can't be countered"
// promises live at once, so this is the ordinary shape, not an edge).
type delayedSpellCastFire struct {
	dt         state.DelayedTrigger
	sa         *cards.SA
	remembered []state.Target
	referents  effects.TriggerContext
	svars      map[string]string
	static     bool
}

func (e *Engine) checkEventDelayedTriggers(ev events.Event, lki *state.Object) {
	var fires []delayedSpellCastFire
	for i := range e.G.Delayed {
		dt := &e.G.Delayed[i]
		// The event-matched modes: SpellCast (a spell's PutOnStack) and
		// ChangesZone (a move, the Earthbend return promise). A Mode$ Phase
		// registration carries no EventMode at all and is owned by
		// checkDelayedTriggers at its phase occurrence.
		if dt.EventMode != "SpellCast" && dt.EventMode != "ChangesZone" && dt.EventMode != "BecomeMonarch" {
			continue
		}
		// The ThisTurn$ mirror: a registration whose expiry turn has passed
		// never fires (see checkDelayedTriggers; skipped, never removed).
		if dt.MaxTurn > 0 && e.G.Turn > dt.MaxTurn {
			continue
		}
		if int(dt.Controller) >= len(e.G.Players) || e.G.Players[dt.Controller].Lost {
			continue
		}
		src := e.G.Obj(dt.Source)
		if src == nil || src.Face() == nil || dt.Trigger == "" {
			continue
		}
		// The stored trigger body: a SVar NAME (the keyword-expansion shape
		// rules.registerOpeningEffectTriggers mints) resolves against the
		// source's own table; an inline body (effDelayedTrigger's Mode$
		// SpellCast branch, a face Ability's DelayedTrigger with no SVar name
		// of its own — Mistrise Village — or an Earthbend registration's
		// stored "Mode$ ChangesZone | ..." body) is stored raw and parses
		// directly. The two carriers cannot collide: no SVar name starts
		// "Mode$".
		raw := dt.Trigger
		if !strings.HasPrefix(raw, "Mode$") {
			raw = src.Face().SVars[raw]
		}
		t, ok := cards.ParseTriggerLine(raw)
		if !ok || t.Mode != dt.EventMode {
			continue
		}
		// referentsArg is the LKI snapshot handed to triggerReferents. The
		// SpellCast arm deliberately passes nil (the spell object itself is
		// the referent source), exactly as it did before the ChangesZone arm
		// existed; the ChangesZone arm threads the leaving object's snapshot
		// so a departing source's last battlefield characteristics are read
		// (CR 603.10a), the same lki the face-trigger walk uses.
		var referentsArg *state.Object
		if dt.EventMode == "BecomeMonarch" {
			// Palace Jailer's command-zone trigger body reads
			// `ValidPlayer$ Player.OpponentOf Remembered` — "until an
			// OPPONENT becomes the monarch". The opponent relation is
			// against the EFFECT's controller (dt.Controller), the player
			// Forge's `RememberObjects$ You & Targeted` anchors. In a
			// multiplayer game that is NOT the exiled creature's own
			// controller: seat 0's Jailer exiles seat 1's creature and seat
			// 2 takes the crown, so the creature must return even though
			// seat 2 does not control it.
			if v := strings.TrimSpace(t.Params["ValidPlayer"]); strings.EqualFold(v, "Player.OpponentOf Remembered") {
				if int(ev.Player) >= len(e.G.Players) || e.G.Players[ev.Player].Lost ||
					!effects.MatchesPlayerSpec(e.G, "Opponent", ev.Player, dt.Controller) {
					continue
				}
				delete(t.Params, "ValidPlayer")
			}
			if !e.becomeMonarchMatches(t, dt.Source, ev) {
				continue
			}
			referentsArg = nil
		} else if dt.EventMode == "ChangesZone" {
			// The Earthbend return promise. destinationAdmits handles the
			// comma-separated Destination$ list (Graveyard,Exile) that
			// zoneChangeMatches reads with the single-word effects.ParseZone
			// -- the engine-wide comma-Destination$ defect, ledgered
			// separately. The special case is local to delayed
			// registrations, so a face trigger keeps the existing single-word
			// reading; a single-zone string is not touched at all.
			if d, ok := t.Params["Destination"]; ok && strings.Contains(d, ",") {
				if !zoneDelayedDestinationAdmits(d, ev.To) {
					continue
				}
				delete(t.Params, "Destination")
			}
			if !e.zoneChangeMatches(t, dt.Source, ev, lki) {
				continue
			}
			referentsArg = lki
		} else if !e.eventDelayedSpellCastMatches(t, dt, ev) {
			continue
		}
		if dt.EventMode != "BecomeMonarch" && !e.triggerConditionHoldsAs(t, dt.Source, dt.Controller) {
			continue
		}
		sa := cards.ResolveSVar(src.Face().SVars, dt.Execute)
		if sa == nil {
			continue
		}
		fires = append(fires, delayedSpellCastFire{
			dt: *dt,
			sa: sa,
			remembered: func() []state.Target {
				if dt.EventMode == "BecomeMonarch" {
					return append([]state.Target(nil), dt.Remembered...)
				}
				return triggerRemembered(ev, dt.Source)
			}(),
			referents: e.triggerReferents(t, dt.Source, ev, referentsArg),
			svars:     src.Face().SVars,
			static:    strings.TrimSpace(t.Params["Static"]) != "",
		})
	}
	// The firing pass runs over the collected copies, never the live slice:
	// a static fire's synchronous emit consumes its registration (splice) and
	// the body's resolution may register or consume further registrations.
	for i := range fires {
		f := &fires[i]
		dt := &f.dt
		// Forge's STATIC delayed trigger (TriggerHandler's isStatic arm): the
		// Execute body resolves IMMEDIATELY at fire time — never pushed on
		// the stack — so the promise it creates (Mistrise Village's
		// "the next spell you cast this turn can't be countered") is
		// active before any player can respond to the cast. The static-
		// marked DelayedPush consumes the one-shot registration without
		// minting a stack object (events.Apply), then the body resolves
		// inline; an ask the body poses parks through the ordinary direct
		// resume machinery.
		if f.static {
			e.emit(events.Event{Kind: events.DelayedPush, Obj: dt.Source,
				Player: dt.Controller, Amount: int32(dt.ID), Counter: dt.Execute,
				Text: "static"})
			ctx := effects.Ctx{Source: dt.Source, Controller: dt.Controller,
				Remembered:     f.remembered,
				Captured:       f.remembered,
				SVars:          f.svars,
				TriggerContext: f.referents,
			}
			effects.Resolve(e, &ctx, f.sa)
			continue
		}
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     dt.Source,
			Controller: dt.Controller,
			Delayed:    true,
			DelayedID:  dt.ID,
			Execute:    dt.Execute,
			SA:         f.sa,
			Ctx: effects.Ctx{
				Source:     dt.Source,
				Controller: dt.Controller,
				Remembered: f.remembered,
				Captured:   f.remembered,
				// The same event-provenance capture the ordinary face
				// SpellCast path takes (triggerReferents' SpellCast case),
				// so the fired ability resolves TriggeredActivator/
				// TriggeredSource exactly as a face trigger would.
				TriggerContext: f.referents,
			},
		})
	}
}

// eventDelayedSpellCastMatches is the event-matched registration's validity
// evaluation, mirroring spellCastMatches' clause grammar (ValidCard$,
// ValidActivatingPlayer$, PlayerTurn$) with one deliberate difference: the
// "you" every player clause is measured against is dt.Controller, the
// registration's effect owner, not the source card's controller (see
// checkEventDelayedTriggers above).
func (e *Engine) eventDelayedSpellCastMatches(t cards.Trigger, dt *state.DelayedTrigger, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	// Casting a spell means an actual card entering the stack (Ruling F3,
	// the same guard spellCastMatches carries).
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	if actionTriggerModes[t.Mode] && strings.EqualFold(t.Params["PlayerTurn"], "True") &&
		e.G.Active != dt.Controller {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		// The cast-provenance qualifiers (castprov1/2/3 — narset's
		// `ValidCard$ Instant.wasCastFromYourHand,Sorcery.wasCastFromYourHand`)
		// split out BEFORE spellCastPermanentSpec rewrites the base: the strip
		// helpers match the raw Forge spec's predicate chain. The spell is on
		// the stack (this is the PutOnStack event), so the log read is honest.
		v, ok := e.castProvenanceAdmits(v, ev.Obj, dt.Controller)
		if !ok {
			return false
		}
		if !effects.MatchesSpecCtx(e.G, spellCastPermanentSpec(v), ev.Obj, e.specCtx(dt.Source, dt.Controller)) {
			return false
		}
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, dt.Controller) {
			return false
		}
	}
	// The same cast-condition clauses spellCastMatches evaluates, mirrored
	// so a stored body carrying either stays fire-time-correct (the "you"
	// the activator clauses measure is the event's caster either way).
	if v, ok := t.Params["ActivatorThisTurnCast"]; ok {
		if !compareIntCount(int32(e.spellsCastThisTurn(ev.Player)), v) {
			return false
		}
	}
	if v, ok := t.Params["ValidSA"]; ok {
		if !e.validSAMatches(dt.Source, ev, dt.Controller, v) {
			return false
		}
	}
	// The target-shape params (targetsvalid1), the delayed mirror: no corpus
	// carrier combines a delayed SpellCast registration with either param
	// (measured: the two files carrying both a DelayedTrigger and a
	// target-shape param carry only Mode$ Phase bodies), but the stored
	// grammar mirrors spellCastMatches' clauses one for one and the read is
	// honest here -- the spell is already on the stack with its recorded
	// targets, so a future registration cannot widen silently.
	if !e.targetShapeMatches(t, obj.Targets, dt.Source, dt.Controller) {
		return false
	}
	return true
}
