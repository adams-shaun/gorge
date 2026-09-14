// altcast.go implements the alternative-cost keyword family: Evoke, Dash,
// Overload, Warp, Madness and Encore, plus the AlternateAdditionalCost
// either-or additional cost. The casting options themselves live where the
// other cast modes do (rules/legal.go offers, rules/cast.go's beginCast
// switch); this file owns the post-cast machinery the keywords imply:
//
//   - Evoke (CR 702.79a: cast for the evoke cost and "when it enters ... its
//     controller sacrifices it" -- the sacrifice is UNCONDITIONAL, there is
//     no pay-to-keep option): a mandatory ETB follow-up queued by
//     checkTriggers when a FlagEvoked creature enters (altCostEnter) and
//     sacrificed at placement by pushTrigger's Evoke arm, inside the ordinary
//     trigger drain. The MH3 non-mana evoke costs (Fury/Grief's
//     ExileFromHand) are paid as Exile cost parts during the cast itself.
//   - Dash (CR 702: "it gains haste, and it's returned from the battlefield
//     to its owner's hand at the beginning of the next end step"): the haste
//     is a UntilEOT layer-6 continuous grant; the return is a Mode$ Phase
//     delayed trigger registered here and resolved through the ordinary
//     DelayedPush machinery with the __kwDashReturn builtin SVar
//     (cards/link.go).
//   - Warp: "exile this creature at the beginning of the next end step,
//     then you may cast it from exile on a later turn" -- a delayed trigger
//     registers the exile (__kwWarpExile), and legal.go's exile walk offers
//     the recast when warpRecastAvailable derives the eligibility from the
//     log (the CastFlags that would mark it reset when the permanent left
//     the battlefield, but the log is permanent).
//   - Madness (CR 702.35a: "If you discard this card, discard it into exile.
//     When you do, cast it for its madness cost or put it into your
//     graveyard"): the discard sites in effects/cardflow.go and rules route
//     a madness card's discard to exile with a "discarded" Text marker; the
//     offer queued here is a pendingTrigger like Miracle's, a yes entering
//     beginCast's "madness" mode and a no putting the card into its
//     owner's graveyard.
//
// All of it is rules-layer on purpose: the pieces need ParseCost, payMana,
// AddContinuous and the pendingTrigger queue, none of which effects may
// reach, and every step is event-emitted (or queue-appended, the same
// memory-only channel checkTriggers already uses) so a replayed game
// re-derives it identically.
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// altCostEnter is called from checkTriggers for every MoveZone onto the
// battlefield. Three keyword follow-ups key off the CastFlags the spell
// carried onto the permanent (a zone change preserves them from the stack):
// an evoked creature queues its pay-or-sacrifice trigger, a dashed creature
// gains haste until end of turn and registers its end-step return, and a
// warped creature registers its end-step exile. Each registration is its own
// DelayedRegister event, so a replay rebuilds the identical set.
//
// Called inside emit (checkTriggers's own context), so the ClockTick
// AddContinuous emits for the haste grant lands between the entering
// MoveZone and whatever follows -- the same nesting emit's Note re-entrancy
// already tolerates.
func (e *Engine) altCostEnter(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Face() == nil {
		return
	}
	if int(o.Controller) >= len(e.G.Players) || e.G.Players[o.Controller].Lost {
		return
	}
	if o.CastFlags&state.FlagEvoked != 0 {
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     ev.Obj,
			Controller: o.Controller,
			Evoke:      true,
			Ctx: effects.Ctx{
				Source:     ev.Obj,
				Controller: o.Controller,
			},
		})
	}
	if o.CastFlags&state.FlagDashed != 0 {
		// CR 702: a dashed spell gains haste. A layer-6 grant scoped to the
		// object itself (the effPump shape), UntilEOT: it leaves with the
		// permanent at the next end step anyway, and the turn boundary
		// drops the grant for the pathological case that it stays.
		e.AddContinuous(state.ContinuousEffect{
			Source: ev.Obj, Affects: "Card.Self", Controller: o.Controller,
			Layer: state.LAbilities, AddKeywords: []string{"Haste"}, UntilEOT: true,
		})
		e.emit(events.Event{Kind: events.DelayedRegister, Obj: ev.Obj,
			Player: o.Controller, Step: state.StepEnd, Counter: "__kwDashReturn"})
	}
	if o.CastFlags&state.FlagWarped != 0 {
		e.emit(events.Event{Kind: events.DelayedRegister, Obj: ev.Obj,
			Player: o.Controller, Step: state.StepEnd, Counter: "__kwWarpExile"})
	}
}

// sacrificeEvoked applies the evoked creature's CR 702.79a sacrifice at the
// follow-up's PLACEMENT (pushTrigger's Evoke arm): the sacrifice is
// unconditional, so there is no decision to pose and the MoveZone is emitted
// right here, inside the ordinary drain -- ordered among the other
// same-controller triggers an ordering ask may have just settled, and never
// before them. It is NOT a stack object: no face T: line backs it (the
// keyword expansion carries none), so the minting machinery a TriggerPush
// needs does not exist for it -- an opponent cannot respond to the sacrifice
// itself, the one fidelity gap this shortcut carries (recorded in the
// ticket report's Issues).
func (e *Engine) sacrificeEvoked(pt pendingTrigger) {
	o := e.G.Obj(pt.Source)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	if int(pt.Controller) >= len(e.G.Players) || e.G.Players[pt.Controller].Lost {
		return
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: pt.Source, From: o.Zone,
		To: state.ZGraveyard, Text: "sacrificed (evoke)"})
}

// offerMadness queues the madness cast offer (CR 702.35a) after a discard
// exiled the card: a pendingTrigger the drain treats exactly like a Miracle
// offer -- an optional yes/no whose decider is the card's owner, a yes
// entering the ordinary cast flow with the "madness" mode. The discard site
// (effects/cardflow.go, the discard costs, the cleanup step) is what exiled
// the card and marked the move "discarded", so matching that marker here is
// the whole of the detection.
func (e *Engine) offerMadness(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Face() == nil {
		return
	}
	if _, ok := o.Face().KeywordParam("Madness"); !ok {
		return
	}
	// Only a DISCARD opens the madness cast window (CR 702.35a: "If a player
	// discards this card ..."); a hand->exile move by any other means (an
	// ExileFromHand cost such as Fury's evoke, a future wheel effect) does
	// not. Every discard site marks its madness move "discarded" through
	// discardEventText/emitDiscard, so the marker is the whole of the
	// discrimination -- without it, evoking Fury while holding a Fiery
	// Temper would falsely offer the Temper's madness cast.
	if !strings.Contains(ev.Text, "discarded") {
		return
	}
	owner := o.Owner
	if int(owner) >= len(e.G.Players) || e.G.Players[owner].Lost {
		return
	}
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Source:     ev.Obj,
		Controller: owner,
		Madness:    true,
		Ctx: effects.Ctx{
			Source:     ev.Obj,
			Controller: owner,
		},
	})
}

// castMadness is the yes answer's placement step (pushTrigger): the card is
// still in exile -- something else may have moved it since the offer was
// queued -- and the ordinary cast flow with Mode "madness" charges the
// printed madness cost. Same drainAwaitsTarget continuation as castMiracle.
func (e *Engine) castMadness(pt pendingTrigger) {
	o := e.G.Obj(pt.Source)
	if o == nil || o.Zone != state.ZExile || o.Controller != pt.Controller {
		// The offer can no longer be honoured; put the card where declining
		// would have (CR 702.35a's "or put it into your graveyard") so it is
		// not stranded in exile by a race it did not choose.
		if o != nil && o.Zone == state.ZExile {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone,
				To: state.ZGraveyard, Text: "madness not cast"})
		}
		return
	}
	e.drainAwaitsTarget = false
	e.beginCast(pt.Controller, decision.Option{Kind: "cast", Obj: pt.Source, Mode: "madness"})
	e.drainAwaitsTarget = e.pending != nil
}

// madnessDeclined is the no answer's placement step (handleTriggerOptional):
// the card has not been cast, so CR 702.35a puts it into its owner's
// graveyard.
func (e *Engine) madnessDeclined(pt pendingTrigger) {
	if o := e.G.Obj(pt.Source); o != nil && o.Zone == state.ZExile {
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone,
			To: state.ZGraveyard, Text: "madness not cast"})
	}
}

// warpRecastAvailable reports whether the warp card id in exile may be cast
// from exile on a later turn (CR 702: "exile this creature at the beginning
// of the next end step, then you may cast it from exile on a later turn").
// It is derived entirely from the log, never from mutable per-object state,
// because the FlagWarped that would carry the provenance resets when the
// permanent left the battlefield at the exile itself. The shape it looks
// for, walking backwards from the log's end:
//
//  1. the most recent MoveZone taking id to exile (the end-step warp exile),
//  2. at least one TurnChange strictly after it ("on a later turn"),
//  3. a DelayedPush of id before that exile (the end-step trigger fired it),
//  4. a CastInfo of id flagged warped before that DelayedPush (it was
//     warp-cast).
//
// A card exiled by anything other than its own warp trigger fails step 3 and
// gets no offer. Everything is a fixed-order walk of the log, so a replayed
// game derives the same answer.
func (e *Engine) warpRecastAvailable(id state.ObjID) bool {
	log := e.L.Events
	exileIdx := -1
	for i := len(log) - 1; i >= 0; i-- {
		ev := log[i]
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZExile {
			exileIdx = i
			break
		}
	}
	if exileIdx < 0 {
		return false
	}
	laterTurn := false
	for i := exileIdx + 1; i < len(log); i++ {
		if log[i].Kind == events.TurnChange {
			laterTurn = true
			break
		}
	}
	if !laterTurn {
		return false
	}
	pushIdx := -1
	for i := exileIdx - 1; i >= 0; i-- {
		ev := log[i]
		if ev.Kind == events.MoveZone && ev.Obj == id {
			// id moved between zones after the DelayedPush we are looking
			// for; keep scanning for the DelayedPush beneath it.
			continue
		}
		if ev.Kind == events.DelayedPush && ev.Obj == id {
			pushIdx = i
			break
		}
	}
	if pushIdx < 0 {
		return false
	}
	for i := pushIdx - 1; i >= 0; i-- {
		ev := log[i]
		if ev.Kind == events.CastInfo && ev.Obj == id &&
			events.FlagsFrom(ev.Counter)&state.FlagWarped != 0 {
			return true
		}
	}
	return false
}

// discardDestZone is the zone a DISCARD of card id sends it to: exile when
// the card carries the Madness keyword (CR 702.35a: "If you discard this
// card, discard it into exile"), the graveyard otherwise. Every discard site
// (effects/cardflow.go's arms, the discard costs in cast.go and
// mana_activation.go, the cleanup step) routes its MoveZone destination
// through this one helper so the replacement cannot be missed by the next
// sibling. Callers pair it with discardEventText, which marks the event so
// offerMadness can tell a discard from any other hand->graveyard move --
// without the marker a plain discard event is indistinguishable from one,
// so the marker is only ever ADDED (existing non-madness discards keep
// emitting exactly the events they always did, keeping every golden replay
// byte-identical).
func discardDestZone(g *state.Game, id state.ObjID) state.Zone {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		if _, ok := o.Face().KeywordParam("Madness"); ok {
			return state.ZExile
		}
	}
	return state.ZGraveyard
}

// discardEventText is the Text a discard MoveZone carries: the "discarded"
// marker (so offerMadness can detect the discard) only for a madness card,
// whose move to exile is the one event the marker disambiguates. A
// non-madness discard keeps the caller's existing Text -- today's sites
// either carry "discarded as a cost" already or carry none -- so no golden
// replay moves.
func discardEventText(g *state.Game, id state.ObjID, base string) string {
	if o := g.Obj(id); o != nil && o.Face() != nil {
		if _, ok := o.Face().KeywordParam("Madness"); ok {
			if base == "" {
				return "discarded (madness)"
			}
			return strings.TrimSuffix(base, " as a cost") + " (madness)"
		}
	}
	return base
}
