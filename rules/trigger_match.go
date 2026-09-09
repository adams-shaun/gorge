// trigger_match.go is the read-only half of the split: checkTriggers walks
// every object once per event and triggerMatches (with its per-kind helpers
// below -- zoneGate, zoneChangeMatches, spellCastMatches, attacksMatches,
// damageMatches, becomesTargetMatches, landPlayedMatches, phaseMatches)
// decides whether a given T: line fires for it. init() registers those
// trigger kinds, and repl:Moved (replacement.go's own kind), as non-API so
// effects.Supported does not mistake them for something a card SVar could
// invoke directly.
//
// Triggered abilities (T: lines) and replacement effects (R: lines). Every
// state mutation still goes through events.Emit -- engine.go's emit wraps it
// with applyReplacements ahead of logging and checkTriggers behind it, so
// this file's job is entirely about *deciding* what fires and *ordering*
// what goes on the stack, never about writing state directly.
package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// pendingTrigger is a matched trigger waiting to be placed on the stack.
//
// Idx is Source's own Face().Triggers index for the matched T: line. SA is
// kept alongside it (the brief's own interface names both Source and SA)
// even though putTriggersOnStack ends up re-deriving the ability from
// Source+Idx rather than using SA directly: Ruling T20-a means the eventual
// stack object is created inside events.Apply, which cannot carry a raw
// *cards.SA pointer through the log -- only data (an ObjID and a small
// index) that lets Apply find the same *cards.SA a live engine already
// holds.
type pendingTrigger struct {
	Source     state.ObjID
	Controller state.PlayerID
	Idx        int
	SA         *cards.SA
	// Miracle marks a keyword offer (Task 18) rather than a matched T: line: a
	// Miracle drawing queued with Idx/SA unset, which the drain treats as an
	// optional trigger whose decider is the owner and routes a yes through
	// castMiracle (miracle.go) instead of minting a triggered-ability stack
	// object. optionalDecider, triggerLabel and pushTrigger all special-case it.
	Miracle bool
	// Delayed marks a Mode$ Phase delayed trigger registration (CR 603.7)
	// rather than a matched T: line. It is queued by checkDelayedTriggers when
	// the registered phase is entered, and pushTrigger routes it to a
	// DelayedPush event instead of a TriggerPush (whose Ability derives from a
	// face Triggers index -- a delayed trigger's effect is an SVar-named
	// sub-ability, not a face trigger). DelayedID is the registration's
	// state.DelayedTrigger.ID (so the fired event can remove exactly it), and
	// Execute is the Execute$ SVar name (which events.Apply's DelayedPush case
	// resolves from the source's SVar table).
	Delayed   bool
	DelayedID uint32
	Execute   string
	Ctx       effects.Ctx
}

// triggerKey identifies one T: line: the object that carries it, plus that
// object's own Triggers index (a card can have more than one). Used both for
// the cascade bound and for DamageDealtOnce's once-per-turn gate.
type triggerKey struct {
	Source state.ObjID
	Idx    int
}

// maxTriggerFires bounds how many times a single (source, trigger index)
// pair may queue a pending trigger over the life of a match. Ordinary play
// stays far below this -- even a trigger that fires every turn for a hundred
// turns is two orders of magnitude under it. The pathological case this
// guards is "a trigger that fires in response to its own effect": nothing in
// this build can hang or overflow the stack resolving any *one* ability
// (Resolve's own maxChain bounds a sub-ability chain, and every stack
// resolution requires a fresh external Submit round-trip -- resolveTop is
// only ever called from handlePriority's "pass" case), but nothing stops a
// naive auto-pass driver from sustaining pop-one/push-one-again forever
// across many such round-trips, tying up the match's one goroutine
// indefinitely. This cap is what makes that terminate: once a specific
// trigger has fired this many times, it simply stops matching, so the loop
// runs dry instead of running forever.
const maxTriggerFires = 256

// forEachObject walks every object currently in the game exactly once, in a
// fixed, deterministic order: living seats in ascending order from seat 0,
// then zone in Zone's own declared order (library, hand, battlefield,
// graveyard, exile, stack), then position within that zone's slice.
// checkTriggers and applyReplacements both need this same walk for their own
// discovery to be deterministic, so it is factored out here rather than
// duplicated. Game.Zone ignores its player argument for ZStack (the stack is
// shared across controllers, not per-seat), so that zone is visited only
// once, on the first living seat, rather than once per living player.
//
// Memory: the live zone slice is copied into e.foreachBuf (engine.go) before
// the walk, because fn can move objects between zones (a trigger match
// putting something on the stack), so iterating the live, mutating slice
// would be a bug. The copy is kept and grown with append(buf[:0], zone...),
// so it settles at the size of the largest zone seen and stops allocating;
// a fresh []state.ObjID allocation per zone per event used to be this
// package's single largest allocation site (Task A2). fn is NEVER called
// after the zone it is walking mutates.
//
// Re-entry safety: each rendering passes its snapshot buffer separately --
// the depth-0 call (from checkTriggers or applyReplacements, always) reuses
// e.foreachBuf; a re-entrant call (fn reaching forEachObject again, directly
// or through emit -> checkTriggers) takes a fresh local buffer instead, so
// the inner walk can never overwrite the outer walk's snapshot mid-range.
// That re-entry is not reachable today -- the only two callers' fns are
// checkTriggers' and applyReplacements', neither of which calls emit or
// forEachObject -- but the guard makes the shape safe if one ever does,
// which is why this file does not rely on "re-entry cannot happen" to keep a
// single shared buffer correct.
func (e *Engine) forEachObject(fn func(id state.ObjID)) {
	e.foreachDepth++
	defer func() { e.foreachDepth-- }()
	buf := e.foreachBuf
	if e.foreachDepth > 1 {
		// Re-entrant: own a private snapshot, never the shared field.
		buf = nil
	}
	for si, p := range e.G.AliveFrom(0) {
		for z := state.ZLibrary; z <= state.ZStack; z++ {
			if z == state.ZStack && si != 0 {
				continue
			}
			// Reassign (not append inline) so the grown snapshot is carried into
			// e.foreachBuf: buf[:0] then append-in-place reuses the backing array
			// from the previous zone / previous call, settling at the largest
			// zone and stopping allocation. Walking the returned slice, not a
			// throwaway append expression, keeps the range over the persisted
			// snapshot rather than a discarded temporary.
			buf = append(buf[:0], e.G.Zone(z, p)...)
			for _, id := range buf {
				fn(id)
			}
		}
	}
	if e.foreachDepth <= 1 {
		// Keep the grown buffer on the Engine for the next depth-0 walk; a
		// re-entrant call's private buffer is discarded on return.
		e.foreachBuf = buf
	}
}

// controllerOf is a nil-safe Object.Controller read: a nonexistent ObjID
// (stale data, a malformed trigger source) degrades to seat 0 rather than
// panicking.
func (e *Engine) controllerOf(id state.ObjID) state.PlayerID {
	if o := e.G.Obj(id); o != nil {
		return o.Controller
	}
	return 0
}

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
		if dt.Phase != ev.Step {
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
			},
		})
	}
}

// triggerSnapshot is immutable look-back state. Parked replacement choices
// may retain it across intent/Clone boundaries; each matching walk constructs
// its own Engine scratch caches, never mutating or sharing the snapshot's.
type triggerSnapshot struct {
	game       *state.Game
	continuous []ContinuousEffect
}

func (e *Engine) snapshotTriggerBoard() *triggerSnapshot {
	return &triggerSnapshot{game: e.G.Clone(), continuous: append([]ContinuousEffect(nil), e.continuous...)}
}

func (e *Engine) checkTriggers(ev events.Event, lki *state.Object) {
	batch := e.triggerBefore != nil && ev.Kind == events.MoveZone &&
		ev.From == state.ZBattlefield && ev.To != state.ZBattlefield
	if batch {
		// Only leaves-the-battlefield triggers look back. Always and other
		// event modes continue to read the live board, not an obsolete state.
		observer := &Engine{G: e.triggerBefore.game, continuous: e.triggerBefore.continuous}
		e.checkFaceTriggers(observer, ev, observer.G.Obj(ev.Obj), true, true)
	}
	e.checkFaceTriggers(e, ev, lki, batch, false)
	if ev.Kind == events.Draw {
		e.offerMiracle(ev)
	}
	if ev.Kind == events.StepChange {
		e.checkDelayedTriggers(ev)
	}
}

// checkFaceTriggers separates the read-only matching board from the live
// queue and firing limits. Both walks use deterministic seat/zone/slice order;
// the ordinary APNAP drain still asks each controller to order their triggers.
func (e *Engine) checkFaceTriggers(observer *Engine, ev events.Event, lki *state.Object, split, leaving bool) {
	observer.forEachObject(func(id state.ObjID) {
		o := observer.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			// Ruling F3: an ability or token object has no Face and
			// therefore no printed Triggers to check -- only real cards
			// carry triggered abilities.
			return
		}
		// objLKI is the whole-event LKI snapshot, hoisted here because every
		// trigger this loop matches for this event shares it (lki.ID == ev.Obj
		// always holds when lki != nil -- see checkTriggers's own doc above; a
		// defensive belt-and-braces check against a future emit change that
		// might one day pass a mismatched lki). triggerMatches and the matched
		// trigger's Ctx both read it.
		var objLKI *state.Object
		if lki != nil && lki.ID == ev.Obj {
			objLKI = lki
		}
		for ti, t := range f.Triggers {
			// A "from anywhere" graveyard trigger is NOT a leaves-the-
			// battlefield trigger (CR 603.6c), even when this particular
			// move happens to leave the battlefield. Only the explicit
			// battlefield-origin shape looks back; destination triggers
			// still use the post-event source/zone in the live walk.
			looksBack := t.Mode == "ChangesZone" && t.Params["Origin"] == "Battlefield"
			if split && looksBack != leaving {
				continue
			}
			// CR 603.8 state trigger: its condition is checked against the
			// current state, not against the event under test, and it fires
			// at most once per outstanding instance. A trigger that already
			// has an instance queued or on the stack does not re-fire, so a
			// condition that stays true cannot enqueue an unbounded run
			// (the concise standing caveat against naively re-firing Always
			// on every bookkeeping event).
			if t.Mode == "Always" && e.stateTriggerOutstanding(id, ti) {
				continue
			}
			if !observer.triggerMatches(t, id, ev, objLKI) {
				continue
			}
			key := triggerKey{Source: id, Idx: ti}
			if e.triggerFireCount == nil {
				e.triggerFireCount = map[triggerKey]int32{}
			}
			if e.triggerFireCount[key] >= maxTriggerFires {
				continue // cascade bound: see maxTriggerFires.
			}
			if t.Mode == "DamageDealtOnce" {
				if e.damageOnceFired == nil {
					e.damageOnceFired = map[triggerKey]int32{}
				}
				if e.damageOnceFired[key] == e.G.Turn {
					continue // already fired this turn.
				}
				e.damageOnceFired[key] = e.G.Turn
			}
			e.triggerFireCount[key]++
			if t.Effect == nil {
				// Execute$ named an SVar this face never defined (or one
				// that failed to parse): the trigger matched, but there is
				// nothing to run.
				continue
			}
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				Idx:        ti,
				SA:         t.Effect,
				Ctx: effects.Ctx{
					Source:         id,
					Controller:     o.Controller,
					Remembered:     triggerRemembered(ev, id),
					LKI:            objLKI,
					TriggerContext: observer.triggerReferents(t, id, ev),
				},
			})
		}
	})
}

// triggerRemembered is what a matched trigger's Ctx.Remembered holds: the
// object the triggering event was actually about (the card that changed
// zones, was cast, or is being targeted), or -- for an event with no object
// of its own, such as a step change, or a player-only Damage event -- the
// trigger's own source. Most of the eight M1 modes' Execute$ abilities in
// the acceptance deck don't read Remembered at all (they use Defined$
// Self/You), so this is deliberately one simple, general rule rather than a
// mode-specific one -- except DeclareAttackers, which carries every attacker
// declared against one defending player in Event.IDs rather than a single
// Event.Obj (see attacksMatches): Remembered there is every declared
// attacker, in order, followed by one more entry for that event's defending
// player. ev.Player is set by handleAttackers (rules/combat.go); since Task
// m34 an attack may split across several defenders, so the engine emits ONE
// DeclareAttackers event per defending player and each event's Player is
// that event's own defender -- a trigger matching one of its attackers
// resolves TriggeredDefendingPlayer against the opponent that creature is
// actually attacking, the same value a two-player game always produced.
//
// effects.context.go's Defined already recognises Defined$
// TriggeredDefendingPlayer/TriggeredPlayer (Task 5's playersOf(Remembered),
// merged after this task started). The trailing entry only matters once it
// survives onto the stack, which needed FL-41's fix: pushTrigger
// (trigger_queue.go) could not just write tgt.Obj for this entry into the
// logged TriggerPush event's IDs (always 0 for a player target) -- Apply
// would rebuild it as {Obj: 0}, and playersOf (effects/context.go) filters
// that entry out, so Defined$ TriggeredDefendingPlayer resolves to nothing
// and the effect silently no-ops (PlayerOf is never reached). state.PlayerRef
// is what lets a player reference survive that log-and-replay round trip
// intact.
//
// The one acceptance-game behaviour this trailing entry actually changes is
// Knight of Infamy's intrinsic Exalted keyword trigger (cards/keywords.go
// expands it to Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True),
// which fires twice in the 4-seat game (mono-black-aggro is dealt in at
// seat 3 there). Exalted's DB$ Pump | Defined$ TriggeredAttacker resolves
// through effects/context.go's objectsOf(c.Remembered): Remembered is every
// declared attacker (plus the defending player). On top of FL-48 (task 16),
// attacksMatches honours Alone$ True, so the trigger now fires only when
// exactly one attacker is declared and pumps that single attacker -- which is
// the measured cause of the four-seat chain-head move 81a8a100641b5442's
// successor (see commit 75be2a3's merge note); before FL-48, Remembered listed
// several attackers and attacksMatches, ignoring Alone$, pumped every one.
// Goblin Guide and Goblin Piledriver never fire in the 8-seat game, and
// Ulamog is in the tron deck, which is never dealt at 2/4/6/8 seats: neither
// is a cause.
func triggerRemembered(ev events.Event, source state.ObjID) []state.Target {
	if ev.Kind == events.DeclareAttackers {
		out := make([]state.Target, 0, len(ev.IDs)+1)
		for _, id := range ev.IDs {
			out = append(out, state.Target{Obj: id})
		}
		return append(out, state.Target{Player: ev.Player, IsPlayer: true})
	}
	if ev.Obj != 0 {
		return []state.Target{{Obj: ev.Obj}}
	}
	return []state.Target{{Obj: source}}
}

// triggerMatches decides whether one cards.Trigger fires for ev. lki is the
// event's LKI snapshot (the moving object as it was a moment ago), threaded
// to a ChangesZone trigger so its ValidCard$ -- a "dies" condition such as
// Undying's counters_EQ0_P1P1 -- can see the object as it was before Move
// reset it, not the live object already in the destination zone.
func (e *Engine) triggerMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if !e.zoneGate(t, source, ev) {
		return false
	}
	var matched bool
	switch t.Mode {
	case "ChangesZone":
		matched = e.zoneChangeMatches(t, source, ev, lki)
	case "SpellCast":
		matched = e.spellCastMatches(t, source, ev)
	case "AbilityCast", "SpellAbilityCast":
		matched = e.abilityCastMatches(t, source, ev)
	case "Attacks":
		matched = e.attacksMatches(t, source, ev)
	case "DamageDone", "DamageDealtOnce":
		matched = e.damageMatches(t, source, ev)
	case "BecomesTarget":
		matched = e.becomesTargetMatches(t, source, ev)
	case "LandPlayed":
		matched = e.landPlayedMatches(t, source, ev)
	case "Phase":
		matched = e.phaseMatches(t, source, ev)
	case "Always":
		// CR 603.8 state trigger: the event under test is irrelevant; the
		// trigger fires when its condition holds (see triggerConditionHolds)
		// and no instance is outstanding (the checkTriggers latch above).
		matched = true
	}
	if !matched {
		return false
	}
	// CR 603.4 intervening-if: a trigger whose condition is false at the
	// moment the trigger event occurs does not trigger at all. This gate is
	// applied uniformly to every mode so the same T: line grammar (a
	// LifeAmount$ or IsPresent$+PresentCompare$ clause on the trigger) is
	// honoured wherever it appears.
	if !e.triggerConditionHolds(t, source) {
		return false
	}
	return true
}

// zoneGate implements TriggerZones$: a trigger only fires while its source is
// in one of the listed zones. The default is the battlefield, which is why an
// enchantment's upkeep trigger stops when it is destroyed.
//
// checkTriggers calls this after the event has already been folded into
// state (emit logs before it checks triggers), so o.Zone alone only ever
// reflects the zone the object is in *now*. That is correct for an
// entering-the-zone trigger (Snapcaster's ETB: o.Zone is already
// Battlefield by the time this runs) but wrong for a leaving-the-zone one --
// a plain "dies" trigger (Origin$ Battlefield, Destination$ Graveyard,
// ValidCard$ Card.Self, default TriggerZones$ Battlefield) would never see
// its own source "in" the battlefield, because by the time checkTriggers
// runs the move has already happened and o.Zone reads Graveyard. CR 603.10's
// full "look back in time" is not modeled, but the one case Task 20's own
// ChangesZone mode needs it for is narrow and self-contained: when the event
// under test is itself the zone change of this trigger's own source (source
// == ev.Obj), the zone it was in immediately before (ev.From) counts as well
// as the zone it is in now, so both an ETB and a dies trigger with the
// ordinary default work from the same rule.
func (e *Engine) zoneGate(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	spec := t.Params["TriggerZones"]
	if spec == "" {
		spec = "Battlefield"
	}
	zones := [2]state.Zone{o.Zone, o.Zone}
	n := 1
	if source == ev.Obj && ev.Obj != 0 &&
		(ev.Kind == events.MoveZone || ev.Kind == events.Draw || ev.Kind == events.PutOnStack) {
		zones[1] = ev.From
		n = 2
	}
	for _, zone := range zones[:n] {
		if zoneSpecContains(spec, zone) {
			return true
		}
	}
	return false
}

// zoneSpecContains reports whether spec (a Forge TriggerZones value -- a
// comma-separated list of zone names, constant for the life of the card)
// lists want. It scans the string by slicing comma-separated parts apart with
// strings.Cut, which shares the backing string and allocates nothing, instead
// of strings.Split (whose []string is a fresh allocation per call). zoneGate
// runs from the per-event trigger walk -- the same hot path Task A2 fixed the
// zone copy in -- so this avoids churning an allocation for every trigger on
// every event. Parts are handed to effects.ParseZone unchanged (it trims each
// name itself), so the result is byte-identical to the old
// strings.Split+TrimSpace+ParseZone loop, including its handling of empty
// leading/trailing/double-separator segments: an empty zone name parses to
// the graveyard, exactly as it always did.
func zoneSpecContains(spec string, want state.Zone) bool {
	// Walk separator-delimited segments by index so that, like strings.Split,
	// a spec ending in a separator still yields a final empty segment (which
	// effects.ParseZone resolves to the graveyard). strings.Cut would drop
	// that trailing Phantom Graveyard part and change behaviour on a malformed
	// spec; slicing keeps every segment while allocating nothing.
	i := 0
	for {
		j := strings.IndexByte(spec[i:], ',')
		var part string
		if j < 0 {
			part = spec[i:]
		} else {
			part = spec[i : i+j]
		}
		if effects.ParseZone(part) == want {
			return true
		}
		if j < 0 {
			return false
		}
		i += j + 1
	}
}

// zoneChangeMatches implements Mode$ ChangesZone. The two keyword triggers
// this task expands through it are both handled here (cards/keywords.go):
// Undying's Origin$ Graveyard -> Destination$ Battlefield return is an
// ordinary ChangeZone, and its ValidCard$ Card.Self+counters_EQ0_P1P1 "no
// counters when it died" is read against the LKI; Evolve's Evolve$ True
// gating (the entering creature's derived power OR toughness must exceed the
// source's, CR 702.99a) is checked below once the rest of the spec matches.
func (e *Engine) zoneChangeMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.MoveZone && ev.Kind != events.Draw && ev.Kind != events.PutOnStack {
		return false
	}
	if o, ok := t.Params["Origin"]; ok && o != "Any" && effects.ParseZone(o) != ev.From {
		return false
	}
	if d, ok := t.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		// The trigger's own source moving (source == ev.Obj) with an LKI
		// snapshot available is a dying card asserting a property about
		// itself, e.g. Undying's counters_EQ0_P1P1: read it against the LKI
		// (what it was the moment before the move reset it), not the live
		// object already in the destination zone.
		if source == ev.Obj && ev.Obj != 0 && lki != nil {
			if !effects.MatchesObjectCtx(e.G, v, lki, e.specCtx(source, e.controllerOf(source))) {
				return false
			}
		} else if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, e.controllerOf(source))) {
			return false
		}
	}
	// Evolve$ True (CR 702.99a): the trigger fires only when the entering
	// creature's derived power OR toughness exceeds the source's, so an
	// equal-or-smaller creature entering does not evolve the source. The
	// ordinary ChangesZone path above (Destination$ Battlefield in the
	// expansion) has already narrowed ev.To, so the extra battlefield guard
	// is belt-and-braces.
	if _, hasEvolve := t.Params["Evolve"]; hasEvolve {
		if ev.To != state.ZBattlefield {
			return false
		}
		if e.Power(ev.Obj) <= e.Power(source) && e.Toughness(ev.Obj) <= e.Toughness(source) {
			return false
		}
	}
	return true
}

// spellCastMatches implements Mode$ SpellCast: ValidCard$ and
// ValidActivatingPlayer$ against a PutOnStack event.
func (e *Engine) spellCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	// Casting a spell means an actual card entering the stack. This build
	// also uses PutOnStack-shaped Move()s for nothing else today (triggered
	// abilities go on the stack via a dedicated TriggerPush event --
	// putTriggersOnStack, above, and events.Apply's TriggerPush case), but a
	// Face()-less object could otherwise satisfy a bare "Any"/"Spell"
	// ValidCard$ regardless (matchesBase's Spell/Any cases don't consult
	// Face()), so this guard holds regardless of how a future ability-object
	// path might reach here. Ruling F3.
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidCard"]; ok {
		if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
			return false
		}
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// attacksMatches implements Mode$ Attacks against a DeclareAttackers event.
// DeclareAttackers carries the attackers declared against ONE defending
// player in one event (IDs; Task m34 emits one event per defender), so like
// every other mode here it fires at most once per event -- a creature
// attacking a single opponent therefore fires exactly once, in its own
// defender's event.
//
// Alone$ True (Exalted's expansion, cards/keywords.go -- Ruling FL-48, which
// was previously a known approximation here) gates the trigger to "exactly
// one attacker declared this combat": Exalted must pump only a single lone
// attacker, and with several declared it must not fire at all. Because this
// fires once per event, the lone-attacker check is len(IDs)==1 and the rest
// of the matching selects that one attacker against ValidCard.
func (e *Engine) attacksMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.DeclareAttackers {
		return false
	}
	if v, ok := t.Params["Alone"]; ok && strings.EqualFold(v, "True") && len(ev.IDs) != 1 {
		return false
	}
	spec, ok := t.Params["ValidCard"]
	if !ok {
		for _, id := range ev.IDs {
			if id == source {
				return true
			}
		}
		return false
	}
	ctrl := e.controllerOf(source)
	for _, id := range ev.IDs {
		if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, ctrl)) {
			return true
		}
	}
	return false
}

// damageSource identifies who dealt a just-emitted Damage event, for
// ValidSource$ matching. events.Event carries no explicit source field for
// Damage -- every Damage event this build emits (effects/damage.go's
// DealDamage/DamageAll) comes from a primitive running inside Resolve,
// called only from resolveTop while the resolving spell or ability is still
// the top of the stack (resolveTop pops it only after Resolve returns), so
// the current stack top is that source for every code path this build has
// today. A future combat-damage implementation, or any Damage emission
// outside ability resolution, would need Event to carry an explicit source
// instead of relying on this.
func (e *Engine) damageSource() state.ObjID {
	if len(e.G.Stack) == 0 {
		return 0
	}
	return e.G.Stack[len(e.G.Stack)-1]
}

// damageMatches implements Mode$ DamageDone and DamageDealtOnce (the once-
// per-turn gate itself lives in checkTriggers, alongside the cascade bound;
// this is purely the per-event parameter match, shared by both modes).
func (e *Engine) damageMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Damage {
		return false
	}
	if strings.EqualFold(t.Params["CombatDamage"], "True") {
		// dealCombatDamage (rules/combat.go) is fully implemented, but
		// events.Event still carries nothing to distinguish combat from
		// noncombat damage at the point a trigger checks it, so a trigger
		// that insists on CombatDamage$ True can never fire yet regardless.
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidSource"]; ok {
		src := e.damageSource()
		if src == 0 || !effects.MatchesSpecCtx(e.G, v, src, e.specCtx(source, ctrl)) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		if ev.Obj != 0 {
			if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
				return false
			}
		} else if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	return true
}

// becomesTargetMatches implements Mode$ BecomesTarget: the trigger's own
// source must be among the chosen targets recorded by a TargetsChosen event
// (rules.handleTarget -- "the target decision being answered").
func (e *Engine) becomesTargetMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	targeted := false
	for _, id := range ev.IDs {
		if id == source {
			targeted = true
			break
		}
	}
	if !targeted {
		return false
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		return effects.MatchesSpecCtx(e.G, v, source, e.specCtx(source, e.controllerOf(source)))
	}
	return true
}

// landPlayedMatches implements Mode$ LandPlayed. This fires on the MoveZone
// hand->battlefield of a land specifically -- not on the separate LandPlayed
// event legal.go's "play_land" case also emits, which carries only a Player
// (no Obj), and so has nothing ValidCard$ could ever match against.
func (e *Engine) landPlayedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.MoveZone || ev.From != state.ZHand || ev.To != state.ZBattlefield {
		return false
	}
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil || !obj.Face().IsLand() {
		return false
	}
	if v, ok := t.Params["ValidCard"]; ok {
		return effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, e.controllerOf(source)))
	}
	return true
}

// phaseMatches implements Mode$ Phase.
func (e *Engine) phaseMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StepChange {
		return false
	}
	want := strings.ToLower(t.Params["Phase"])
	if want != "" && !strings.Contains(ev.Step.String(), want) {
		return false
	}
	if v, ok := t.Params["ValidPlayer"]; ok {
		// StepChange carries no Player of its own -- a step always belongs
		// to the current active player.
		if !effects.MatchesPlayerSpec(e.G, v, e.G.Active, e.controllerOf(source)) {
			return false
		}
	}
	return true
}

// abilityCastMatches implements Mode$ AbilityCast and Mode$ SpellAbilityCast
// against an AbilityPush event -- the moment an activated ability is put on
// the stack (rules/cast.go's commitCast). This is the COMPLETED boundary: an
// AbilityPush is emitted only once the activation's cost is fully paid and
// the ability object is minted, so a trigger firing here is never observing a
// provisional or abandoned activation. (F15: there was no such arm at all, so
// Rings of Brighthearth's "Whenever you activate an ability, if it isn't a
// mana ability..." trigger never fired.)
//
// ValidActivatingPlayer$ and ValidSA$ narrow the activation the trigger
// observes, in the same two params the corpus spells them with. A source
// permanent whose face has no Abilities at the recorded index (stale data) is
// a no-op, never a panic.
func (e *Engine) abilityCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.AbilityPush {
		return false
	}
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		// ev.Player is the player who activated the ability;
		// MatchesPlayerSpec resolves "You" as the trigger's controller.
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	if v, ok := t.Params["ValidSA"]; ok {
		if ev.Amount < 0 || int(ev.Amount) >= len(obj.Face().Abilities) {
			return false
		}
		if !abilityCastValidSA(obj.Face().Abilities[int(ev.Amount)], v) {
			return false
		}
	}
	return true
}

// abilityCastValidSA reports whether an activated ability matches a ValidSA$
// narrowing on an AbilityCast/SpellAbilityCast trigger. The grammar is Forge's
// comma-separated OR list of "<kind>.<constraint>" values; a value whose kind
// names a spell (Spell/Instant/Sorcery) describes a cast, not an activation,
// so it never matches an activated ability and is simply skipped. An absent
// or unqualified value matches every activated ability.
func abilityCastValidSA(ab *cards.SA, validSA string) bool {
	v := strings.TrimSpace(validSA)
	if v == "" {
		return true
	}
	for _, alt := range strings.Split(v, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		kind, constraint := alt, ""
		if i := strings.IndexByte(alt, '.'); i >= 0 {
			kind, constraint = alt[:i], alt[i+1:]
		}
		switch kind {
		case "SpellAbility", "Activated", "":
			switch constraint {
			case "":
				return true
			case "!ManaAbility":
				if ab.API != "Mana" {
					return true
				}
			case "ManaAbility":
				if ab.API == "Mana" {
					return true
				}
			}
		}
	}
	return false
}

// triggerConditionHolds evaluates the CR 603.4 intervening-if clause (and the
// CR 603.8 state-trigger condition) carried on a T: line. Two clause shapes
// are recognised, the two the corpus uses on the trigger lines this engine
// routes through the modes above:
//
//   - LifeAmount$ <op><n> against LifeTotal$ (<who>): the named player's life
//     total compared to n.
//   - IsPresent$ <spec> with PresentCompare$ <op><n>: the count of objects
//     matching <spec> compared to n.
//
// An absent clause is vacuously true. A clause whose shape this build cannot
// evaluate FAILS CLOSED -- a false condition means the trigger simply does not
// fire, never that an unreadable life/creature count is presumed large
// enough to let a win or counter trigger slip through.
func (e *Engine) triggerConditionHolds(t cards.Trigger, source state.ObjID) bool {
	if v, ok := t.Params["LifeAmount"]; ok {
		if !e.lifeConditionHolds(t, source, v) {
			return false
		}
	}
	if spec, ok := t.Params["IsPresent"]; ok {
		cmp, ok := t.Params["PresentCompare"]
		if !ok {
			return false
		}
		if !e.presentConditionHolds(t, source, spec, cmp) {
			return false
		}
	}
	return true
}

// lifeConditionHolds evaluates the LifeTotal$/LifeAmount$ intervening-if.
// The "you" for a You-qualified LifeTotal$ is the trigger's controller
// (source's controller), matching how every other trigger param resolves it.
func (e *Engine) lifeConditionHolds(t cards.Trigger, source state.ObjID, amount string) bool {
	who := e.controllerOf(source)
	if v, ok := t.Params["LifeTotal"]; ok {
		v = strings.TrimSpace(v)
		switch v {
		case "", "You":
			// the controller, which who already is
		case "ActivePlayer":
			who = e.G.Active
		default:
			return false // unevaluable player selector: fail closed
		}
	}
	if int(who) >= len(e.G.Players) || e.G.Players[who].Lost {
		return false
	}
	return compareLife(e.G.Players[who].Life, amount)
}

// presentConditionHolds evaluates the IsPresent$/PresentCompare$ intervening-
// if by counting the objects on the battlefield that match the spec (relative
// to the trigger's source and its controller) and comparing that count.
func (e *Engine) presentConditionHolds(t cards.Trigger, source state.ObjID, spec, cmp string) bool {
	n := e.countPresent(spec, source, e.controllerOf(source))
	return comparePresent(n, cmp)
}

// countPresent walks every object on the battlefield once and counts those
// matching spec, excluding the source where the spec's own Other/StrictlyOther
// predicate already handles it (Emperor Crocodile's Creature.Other+YouCtrl).
func (e *Engine) countPresent(spec string, source state.ObjID, you state.PlayerID) int {
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, you)) {
			n++
		}
	})
	return n
}

// compareLife compares a life total against a Forge comparison literal such as
// GE40, EQ0, LE3. Any shape this build cannot fold (a non-numeric rhs, a
// missing operator) is false, so an unreadable condition never fires a
// trigger.
func compareLife(have int32, cmp string) bool {
	op, n, ok := splitCompare(strings.TrimSpace(cmp))
	if !ok {
		return false
	}
	return applyCompare(int(have), op, n)
}

// comparePresent compares a present-count against the same comparison literal
// grammar. A non-numeric rhs (PresentCompare$ EQX) fails closed.
func comparePresent(have int, cmp string) bool {
	op, n, ok := splitCompare(strings.TrimSpace(cmp))
	if !ok {
		return false
	}
	return applyCompare(have, op, n)
}

// splitCompare separates a Forge comparison literal ("GE40", "EQ0") into its
// two-character operator and its numeric rhs. ok is false for anything that is
// not a recognised operator followed by an integer.
func splitCompare(cmp string) (op string, n int, ok bool) {
	if len(cmp) < 3 {
		return "", 0, false
	}
	op = cmp[:2]
	num, err := strconv.Atoi(cmp[2:])
	if err != nil {
		return "", 0, false
	}
	return op, num, true
}

func applyCompare(have int, op string, n int) bool {
	switch op {
	case "GE":
		return have >= n
	case "LE":
		return have <= n
	case "EQ":
		return have == n
	case "GT":
		return have > n
	case "LT":
		return have < n
	case "NE":
		return have != n
	}
	return false
}

// stateTriggerOutstanding reports whether a state trigger (Mode$ Always)
// already has an instance where one would be enqueued -- either still in the
// pending queue or already placed on the stack. This is the CR 603.8 latch:
// it is what stops a continuously-true condition from enqueuing an unbounded
// run of the same trigger.
func (e *Engine) stateTriggerOutstanding(source state.ObjID, idx int) bool {
	for _, pt := range e.pendingTriggers {
		if pt.Source == source && pt.Idx == idx {
			return true
		}
	}
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	f := o.Face()
	if f == nil || idx < 0 || idx >= len(f.Triggers) {
		return false
	}
	sa := f.Triggers[idx].Effect
	for _, sid := range e.G.Stack {
		so := e.G.Obj(sid)
		if so != nil && so.Source == source && so.Ability == sa {
			return true
		}
	}
	return false
}

func init() {
	effects.RegisterNonAPI(
		"trig:ChangesZone", "trig:SpellCast", "trig:Attacks", "trig:DamageDone",
		"trig:DamageDealtOnce", "trig:BecomesTarget", "trig:LandPlayed", "trig:Phase",
		"trig:AbilityCast", "trig:SpellAbilityCast", "trig:Always",
		"repl:Moved",
		// Task 16 keyword triggers, expanded by cards/keywords.go into ordinary
		// ChangesZone / Attacks / SpellCast triggers routed through the modes
		// above: Undying and Evolve are ChangesZone triggers, Exalted is
		// (Alone$) Attacks, Prowess is SpellCast.
		"kw:Undying", "kw:Evolve", "kw:Exalted", "kw:Prowess",
		// Task 17: Storm's expansion (cards/keywords.go) is a SpellCast
		// trigger whose effect is CopySpellAbility -- the expansion existed
		// since Task 11; registering the keyword here completes its
		// semantics now that api:CopySpellAbility is implemented.
		"kw:Storm",
	)
}
