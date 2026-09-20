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
	"slices"
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
	// Madness marks the mandatory triggered ability created after its owner
	// accepts the optional discard-to-exile replacement (CR 702.35a-b). Unlike
	// Miracle it is pushed unconditionally and asks whether to cast only when
	// the respondable ability resolves.
	Miracle bool
	Madness bool
	// Evoke marks the CR 702.79a mandatory sacrifice follow-up queued by
	// altCostEnter. It has no yes/no choice; pushTrigger mints a real
	// respondable keyword-triggered ability on the stack.
	Evoke bool
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
	// Granted marks a static-grant's trigger (AddTrigger$ on a Mode$
	// Continuous static, e.g. Hearthhull's "STATION 8+ Whenever you sacrifice
	// a land"): like a delayed trigger its Ability is an SVar-named body
	// (the Execute$ name rides the GrantTriggerPush event for events.Apply
	// to resolve from the affected object's SVar table), but unlike a
	// delayed registration nothing is consumed -- the grant lives exactly as
	// long as its granting static.
	Granted bool
	Execute string
	// Ward is a GRANTED ward keyword (a layer-6 AddKeyword$ Ward:<cost>, e.g.
	// Hexing Squelcher's "Other creatures you control have 'Ward—Pay 2
	// life.'"): the trigger exists only in the layer system, never on the
	// face, so the queue carries the ward COST text and the drain pushes a
	// KeywordTriggerPush whose __kwWard: payload events.Apply rebuilds the
	// same DB$ Ward ability from. Idx and SA are unset for it.
	Ward string
	// Afflict is a GRANTED afflict keyword (a layer-6 AddKeyword$
	// Afflict:<N>, e.g. Lost Monarch of Ifnir's "Other Zombies you control
	// have afflict 3"): the same shape as Ward -- the queue carries the life
	// amount and the drain pushes a KeywordTriggerPush whose __kwAfflict:
	// payload events.Apply rebuilds into the same DB$ LoseLife body the
	// printed K:Afflict expansion carries. Idx and SA are unset for it.
	Afflict string
	// RingEmblem is one of the Ring emblem's four level abilities (CR
	// 701.54c), queued by checkRingEmblemTriggers. The emblem has no face
	// and no object in any zone, so like Ward/Afflict this entry carries
	// only the LEVEL: pushTrigger mints a RingEmblemPush whose __ring:<level>
	// payload events.Apply rebuilds the ability from. Source is 0 (there is
	// no permanent), Controller the tempted seat.
	RingEmblem int
	Ctx        effects.Ctx
}

// triggerKey identifies one T: line: the object that carries it, plus that
// object's own Triggers index (a card can have more than one). Used both for
// the cascade bound and for DamageDealtOnce/DamageDoneOnce's once-per-batch
// gate.
type triggerKey struct {
	Source state.ObjID
	Idx    int
	// Face distinguishes a Room's two faces (rules/rooms.go): an unlocked
	// room's ALTERNATE face (index 1) carries its own triggers whose
	// fire-count and once-per-turn memory must never share an entry with
	// the cast face's same-index trigger. Zero for every ordinary read --
	// the zero value keeps the field invisible to every existing key build.
	Face uint8
}

// damageBatchKey identifies one DamageDealtOnce/DamageDoneOnce trigger's
// referent within one damage batch. DamageDealtOnce latches per DEALING
// source (Forge GameAction.triggerDamageDoneOnce's dealt half: one trigger per
// source per batch, its referent amount the total that source dealt in the
// batch); DamageDoneOnce latches per DAMAGED object (the done half: one
// trigger per target, its referent amount the total that target took). The
// embedded triggerKey keeps two T: lines of one card -- and the same line on
// two cards -- independent.
type damageBatchKey struct {
	triggerKey
	dealt  bool           // true: referent is the dealing source (DamageDealtOnce)
	obj    state.ObjID    // the referent object (dealing source, or damaged object)
	player state.PlayerID // the referent player when the damage went to a player
}

// damageBatchEntry records one (trigger, referent) pair already queued inside
// the open damage batch: the pendingTriggers index it queued at (the queue is
// append-only while a batch is open, so the index is stable until batch close)
// and the batch amount accumulated so far, which closeDamageBatch patches into
// the queued trigger's TriggerAmount referent.
type damageBatchEntry struct {
	key    damageBatchKey
	idx    int
	amount int32
}

// turnFires is one T: line's trigger count within the turn it last
// triggered (Engine.triggerTurnFires).
type turnFires struct {
	Turn int32
	N    int32
}

// actionTriggerModes are the event-trigger modes the sacrifice/discard/tap/
// crime/attack-declaration ticket registered. The trigger-level parameters
// only they honour -- ActivationLimit$, PlayerTurn$, and a CheckDefinedPlayer$
// predicate this build cannot evaluate failing closed -- are scoped to these
// modes, so no trigger of another mode that fired before stops firing or
// fires less often.
var actionTriggerModes = map[string]bool{
	"AttackersDeclaredOneTarget": true, "AttackersDeclared": true, "Sacrificed": true, "Discarded": true,
	"CommitCrime": true, "Taps": true, "TapsForMana": true,
	// DamagePreventedOnce joins them for the same reason: it is an event mode
	// registered from the start (rules/trigger_match.go's
	// damagePreventedMatches), so the trigger-level parameters Forge scopes
	// to every event mode -- PlayerTurn$, ActivationLimit$, and an
	// unevaluable CheckDefinedPlayer$ predicate failing closed -- apply from
	// day one. No Once latch rides it: each stored prevention Note is one
	// occurrence (prevention happens per Damage event, no batching concept --
	// the DamageDealtOnce/DamageDoneOnce batch latch exists because combat
	// batches several Damage events).
	// TokenCreated/TokenCreatedOnce are event modes registered from the start
	// (rules/trigger_match.go's tokenCreatedMatches), so the trigger-level
	// parameters Forge scopes to every event mode -- PlayerTurn$,
	// ActivationLimit$, and an unevaluable CheckDefinedPlayer$ predicate
	// failing closed -- apply from day one, and the Once mode's implicit
	// once-per-turn latch rides the same queue-time gate.
	"TokenCreated": true, "TokenCreatedOnce": true,
	// ChangesZoneAll joins them for the trigger-level parameters Forge scopes
	// to every event mode: ActivationLimit$ ("triggers only once each turn"
	// on 40 of the 126 corpus ChangesZoneAll lines) MUST be enforced, and the
	// mode's 7 PlayerTurn$ True lines want the same requirement as every
	// other event mode. Membership also makes an unevaluable CheckDefinedPlayer$
	// predicate fail closed for the mode, which is the conservative direction
	// for a mode registered from the start.
	// FlippedCoin joins them for the same reason: it is an event mode
	// registered from the start (rules/trigger_match.go's
	// flippedCoinMatches, firing off the canonical coin-flip result Note
	// both api:FlipCoin and the cumulative-upkeep cost action emit), so the
	// trigger-level parameters Forge scopes to every event mode --
	// PlayerTurn$, ActivationLimit$, and an unevaluable CheckDefinedPlayer$
	// predicate failing closed -- apply from day one. No Once latch: each
	// flip result Note is one occurrence.
	"FlippedCoin":    true,
	"ChangesZoneAll": true,
	// Attached is an event mode registered from the start
	// (attachedMatches over events.Attach), so the trigger-level parameters
	// Forge scopes to every event mode -- PlayerTurn$, ActivationLimit$, and
	// an unevaluable CheckDefinedPlayer$ predicate failing closed -- apply
	// from day one (Inchblade Companion carries ActivationLimit$ 1).
	"Attached": true,
}

// triggerActivationLimitAllows enforces ActivationLimit$ N ("this ability
// triggers only once each turn"): the T: line triggers at most N times per
// turn, counted when it triggers (Forge Trigger.checkActivationLimit and
// TriggerHandler.runSingleTrigger). A malformed limit fails closed. The count
// is recorded here, on the path that is about to queue the trigger.
func (e *Engine) triggerActivationLimitAllows(t cards.Trigger, key triggerKey) bool {
	raw, ok := t.Params["ActivationLimit"]
	if !ok {
		// Mode$ TokenCreatedOnce is Forge's own once-per-turn gate (Akim, the
		// Soaring Wind: "whenever you create one or more tokens for the first
		// time each turn"): an implicit ActivationLimit 1, latched at queue
		// time on the same per-turn map so a batch of mints fires once and
		// the next turn resets. Reusing triggerTurnFires (which Clone already
		// deep-copies) instead of a second latch field keeps the two
		// per-turn trigger counts structurally identical.
		if t.Mode == "TokenCreatedOnce" {
			raw, ok = "1", true
		} else {
			return true
		}
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 0 {
		return false
	}
	// The Once mode's meaning is once per turn; an explicit ActivationLimit$
	// above 1 on a TokenCreatedOnce line cannot raise it.
	if t.Mode == "TokenCreatedOnce" && limit > 1 {
		limit = 1
	}
	if e.triggerTurnFires == nil {
		e.triggerTurnFires = map[triggerKey]turnFires{}
	}
	f := e.triggerTurnFires[key]
	if f.Turn != e.G.Turn {
		f = turnFires{Turn: e.G.Turn}
	}
	if int(f.N) >= limit {
		return false
	}
	f.N++
	e.triggerTurnFires[key] = f
	return true
}

// resolvedLimitValue parses a trigger's ResolvedLimit$ param. ok is true only
// when the param is present; a malformed value is reported as (0, true) so
// the gate denies it (fail closed), matching triggerActivationLimitAllows's
// malformed handling.
func resolvedLimitValue(t cards.Trigger) (int, bool) {
	raw, present := t.Params["ResolvedLimit"]
	if !present {
		return 0, false
	}
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit < 0 {
		return 0, true // malformed: present, denies
	}
	return limit, true
}

// noteTriggerResolved increments source's per-turn resolution count for the
// ResolvedLimit$ gate. Callers MUST have verified that the trigger line being
// resolved itself carries ResolvedLimit$ (resolvedLimitValue on the trigger
// findTriggerForAbility returned for the resolving ability) -- a mixed-line
// source's OTHER trigger lines must never consume this line's limit (the
// round-2 defect: sourceHasResolvedLimit potted any line of the source and
// killed Cosmic Crucible's copy trigger every turn its mandatory Main1 mana
// trigger resolved). The count self-resets when the turn changes, exactly as
// triggerTurnFires does, so no reset hook is needed.
func (e *Engine) noteTriggerResolved(source state.ObjID) {
	if e.triggerTurnResolved == nil {
		e.triggerTurnResolved = map[state.ObjID]turnFires{}
	}
	f := e.triggerTurnResolved[source]
	if f.Turn != e.G.Turn {
		f = turnFires{Turn: e.G.Turn}
	}
	f.N++
	e.triggerTurnResolved[source] = f
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

func (e *Engine) checkEventDelayedTriggers(ev events.Event) {
	var fires []delayedSpellCastFire
	for i := range e.G.Delayed {
		dt := &e.G.Delayed[i]
		if dt.EventMode != "SpellCast" {
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
		// of its own — Mistrise Village) is stored raw and parses directly.
		// The two carriers cannot collide: no SVar name starts "Mode$".
		raw := dt.Trigger
		if !strings.HasPrefix(raw, "Mode$") {
			raw = src.Face().SVars[raw]
		}
		t, ok := cards.ParseTriggerLine(raw)
		if !ok || t.Mode != "SpellCast" {
			continue
		}
		if !e.eventDelayedSpellCastMatches(t, dt, ev) {
			continue
		}
		if !e.triggerConditionHoldsAs(t, dt.Source, dt.Controller) {
			continue
		}
		sa := cards.ResolveSVar(src.Face().SVars, dt.Execute)
		if sa == nil {
			continue
		}
		fires = append(fires, delayedSpellCastFire{
			dt:         *dt,
			sa:         sa,
			remembered: triggerRemembered(ev, dt.Source),
			referents:  e.triggerReferents(t, dt.Source, ev, nil),
			svars:      src.Face().SVars,
			static:     strings.TrimSpace(t.Params["Static"]) != "",
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
	return true
}

// spellCastPermanentSpec rewrites the leading `Permanent` base token of
// every comma-alternative in a SpellCast trigger's ValidCard$ spec to
// `PermanentCard`, so the "whenever you cast a permanent spell" family
// (Unbound Flourishing, the Defiler cycle, Archmage of Echoes) reads the
// base as a permanent card at PutOnStack -- the evaluated object there is
// always the cast spell, and CR 109.2 makes an artifact/creature/
// enchantment/planeswalker/battle spell a permanent spell. matchesBase's
// bare `Permanent` keeps the on-the-battlefield reading every other filter
// depends on and is deliberately untouched. rules/stack.go's
// targetSpecForZone and effects/cardflow.go's permanentCardSpec carry the
// same leading-token rule with a zone gate (both leave ZStack unchanged);
// this third copy applies it WITHOUT a zone gate and ACROSS alternatives,
// because a comma-separated spec names each alternative's own base
// (Archmage of Echoes' `Permanent.Faerie,Permanent.Wizard`). effects must
// not import rules and vice versa, so the copies cannot share code. The
// rewritten text is not the original spec, so the compiled sidecar's
// byText lookup misses and the textual oracle answers it -- which after the
// matchesBase/matchesCompiledBase relaxation is correct.
func spellCastPermanentSpec(spec string) string {
	// Comma-split at angle-bracket depth 0: a named<X, Y> name argument's
	// printed comma is not an alternative boundary (the same distinction
	// effects.filterAlternatives draws).
	depth, start := 0, 0
	var b strings.Builder
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if start > 0 {
					b.WriteByte(',')
				}
				b.WriteString(spellCastLeadingPermanentToCard(spec[start:i]))
				start = i + 1
			}
		}
	}
	if start == 0 {
		return spellCastLeadingPermanentToCard(spec)
	}
	b.WriteByte(',')
	b.WriteString(spellCastLeadingPermanentToCard(spec[start:]))
	return b.String()
}

// spellCastLeadingPermanentToCard is the leading-token rule targetSpecForZone
// and permanentCardSpec share: the whole spec is `Permanent`, or its leading
// token is `Permanent` followed by '.', '+' or ',' -- only that token is
// rewritten, every qualifier rides along.
func spellCastLeadingPermanentToCard(alt string) string {
	if alt == "Permanent" {
		return "PermanentCard"
	}
	if len(alt) > len("Permanent") && alt[:len("Permanent")] == "Permanent" {
		switch alt[len("Permanent")] {
		case '.', '+':
			return "PermanentCard" + alt[len("Permanent"):]
		}
	}
	return alt
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

func (e *Engine) checkTriggers(ev events.Event, lki *state.Object,
	lkiPower, lkiToughness int32, lkiPTValid bool) {
	batch := e.triggerBefore != nil && ev.Kind == events.MoveZone &&
		ev.From == state.ZBattlefield && ev.To != state.ZBattlefield
	if batch {
		// Only leaves-the-battlefield triggers look back. Always and other
		// event modes continue to read the live board, not an obsolete state.
		observer := &Engine{G: e.triggerBefore.game, L: e.L,
			continuous: e.triggerBefore.continuous, continuousVersion: e.continuousVersion}
		obj := observer.G.Obj(ev.Obj)
		var power, toughness int32
		valid := obj != nil && obj.Zone == state.ZBattlefield && obj.Face() != nil
		if valid {
			power, toughness = observer.Power(ev.Obj), observer.Toughness(ev.Obj)
		}
		e.checkFaceTriggers(observer, ev, obj, power, toughness, valid, true, true)
	}
	e.checkFaceTriggers(e, ev, lki, lkiPower, lkiToughness, lkiPTValid, batch, false)
	if ev.Kind == events.PutOnStack {
		e.checkEventDelayedTriggers(ev)
	}
	// Sagas (kw:Chapter): a lore counter's chapter ability queues off the
	// two events that place lore counters -- the battlefield-entry Move
	// (whose own grant is already folded into the live counter the check
	// reads) and a LORE CounterChange (the draw-step half).
	e.checkChapterTriggers(ev)
	if ev.Kind == events.Draw {
		e.offerMiracle(ev)
	}
	// The alternative-cost keyword family's event hooks (altcast.go): a
	// battlefield entry is where an evoked creature queues its pay-or-sacrifice
	// follow-up and a dashed/warped creature registers its end-step delayed
	// trigger; a discard that exiled a madness card queues its cast offer.
	if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield {
		e.altCostEnter(ev)
	}
	if ev.Kind == events.MoveZone && ev.From == state.ZHand && ev.To == state.ZExile {
		e.offerMadness(ev)
	}
	if ev.Kind == events.StepChange {
		e.checkDelayedTriggers(ev)
	}
	// Become-blocked triggers (trig:AttackerBlocked; She-Hulk, Wallbreaker):
	// one queue entry per blocked attacker, which the ordinary per-trigger
	// face scan cannot express (see checkAttackerBlockedTriggers).
	if ev.Kind == events.DeclareBlockers {
		e.checkAttackerBlockedTriggers(ev)
	}
	// Rooms (CR 309.5): the unlocked half's "When you unlock this door"
	// trigger queues off the DoorUnlock event itself -- its face is the
	// alternate face (FaceIdx 1), which the ordinary face scan above does
	// not walk.
	if ev.Kind == events.DoorUnlock {
		e.checkUnlockTriggers(ev)
	}
	// Exert's Trigger$ rider (task exert1, CR 702.100a): the static's named
	// SVar body queues off the Exert event itself, with Source = the
	// exerted permanent. The Amount -1 consume marker fires nothing: it is
	// the untap-step scan's own bookkeeping fold.
	if ev.Kind == events.Exert && ev.Amount >= 0 {
		e.checkExertTriggers(ev)
	}
	// The Ring emblem's four level abilities (CR 701.54c): engine-side
	// "whenever" abilities with no corpus script text and no object in any
	// zone, so the ordinary per-face walk above can never see them. The
	// scan reads state.Player.RingTempted (the fold RingTemptsYou just made
	// renders here BEFORE any trigger check, so level 4 sees its own
	// temptation) and queues one entry per firing seat.
	e.checkRingEmblemTriggers(ev)
}

// The Ring emblem's four level gates (CR 701.54c). Level N is active iff the
// tempted seat's RingTempted >= N; the count is uncapped, so lower levels
// stay active as it rises.
const (
	ringEmblemLevelDraw      = 1 // whenever your Ring-bearer attacks, draw a card
	ringEmblemLevelBlocked   = 2 // whenever your Ring-bearer becomes blocked, discard a card; if you can't, sacrifice it
	ringEmblemLevelCombatHit = 3 // whenever your Ring-bearer deals combat damage to a player, sacrifice it
	ringEmblemLevelTempted   = 4 // whenever the Ring tempts you, each opponent loses 1 life
)

// checkRingEmblemTriggers queues the Ring emblem's level abilities (CR
// 701.54c) for one event. The emblem is not an object in any zone, so the
// ordinary per-face walk cannot reach it; this is the per-event synthetic
// scan site (the checkAttackerBlockedTriggers / checkChapterTriggers
// precedent). Each level names one event shape:
//
//	1  DeclareAttackers whose attacker list holds a seat's Ring-bearer
//	2  DeclareBlockers whose blocked-attacker pairs hold a seat's Ring-bearer
//	3  a COMBAT Damage event whose source is a seat's Ring-bearer and whose
//	   recipient is a player (Obj 0 -- a hit redirected onto a permanent is
//	   not "damage to a player")
//	4  the RingTemptsYou event itself, for the tempted seat
//
// Because RingTemptsYou's own fold runs before checkTriggers, level 4's gate
// reads the POST-fold count: the 4th temptation fires it and the 3rd does
// not. The entries carry no Source (there is no permanent): pushTrigger mints
// a RingEmblemPush whose payload events.Apply rebuilds the ability from.
func (e *Engine) checkRingEmblemTriggers(ev events.Event) {
	switch ev.Kind {
	case events.DeclareAttackers, events.DeclareBlockers, events.Damage, events.RingTemptsYou:
	default:
		return
	}
	for p := state.PlayerID(0); int(p) < len(e.G.Players); p++ {
		if e.G.Players[p].Lost {
			continue
		}
		tempted := e.G.Players[p].RingTempted
		if tempted <= 0 {
			continue
		}
		bearer := e.G.Players[p].RingBearer
		switch ev.Kind {
		case events.RingTemptsYou:
			if ev.Player == p && tempted >= ringEmblemLevelTempted {
				e.queueRingEmblem(p, ringEmblemLevelTempted, bearer)
			}
		case events.DeclareAttackers:
			if tempted >= ringEmblemLevelDraw && bearer != 0 && objIDIn(ev.IDs, bearer) {
				e.queueRingEmblem(p, ringEmblemLevelDraw, bearer)
			}
		case events.DeclareBlockers:
			if tempted >= ringEmblemLevelBlocked && bearer != 0 && blockedAttackerIn(ev.Pairs, bearer) {
				e.queueRingEmblem(p, ringEmblemLevelBlocked, bearer)
			}
		case events.Damage:
			if tempted >= ringEmblemLevelCombatHit && bearer != 0 && ev.Obj == 0 &&
				e.combatDamaging && e.damaging == bearer {
				e.queueRingEmblem(p, ringEmblemLevelCombatHit, bearer)
			}
		}
	}
}

// queueRingEmblem appends one emblem level ability to the pending queue. The
// TriggerCard role names the Ring-bearer the firing event was about so a
// chained body that ever wants it can read it; the hand-built bodies read
// only Defined$ You/Opponent and the live designation.
func (e *Engine) queueRingEmblem(p state.PlayerID, level int, bearer state.ObjID) {
	e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
		Controller: p,
		RingEmblem: level,
		Ctx: effects.Ctx{
			Controller:     p,
			TriggerContext: effects.TriggerContext{TriggerCard: bearer},
		},
	})
}

func blockedAttackerIn(pairs [][2]state.ObjID, id state.ObjID) bool {
	for _, pr := range pairs {
		if pr[0] == id {
			return true
		}
	}
	return false
}

// checkExertTriggers queues the Trigger$ rider of every offerable
// stat:OptionalAttackCost static carried by the exerted permanent (task
// exert1, CR 702.100a "When you do, ..."): a real triggered ability the
// ordinary APNAP drain places with Source = the exerted permanent, resolving
// with the registered APIs every rider body uses (UntapAll, AddPhase, Pump,
// ... -- the triage census). The queue entry is the granted-trigger shape
// (checkGrantedStaticTriggersUsing's precedent): the stack object is minted
// inside events.Apply from the Execute$ SVar name resolved against the
// exerted permanent's own table -- the same body grantedTriggerExecute links
// here, so live queue and replayed log carry the identical SA. The gate
// (ValidCard$, IsPresent$/...) is re-read per static exactly as the offer
// walk (rules/combat.go exertOfferHolds) read it; the static's parameters a
// fire-time re-check cannot evaluate fail closed and queue nothing. The walk
// is the deterministic activeStatics order, never a map, and the cascade
// bound is the granted slot's triggerKey (Source, Idx -1), the same key the
// granted walk shares -- cascade bound only, never once-per-turn memory.
func (e *Engine) checkExertTriggers(ev events.Event) {
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		return
	}
	for _, sv := range e.activeStatics("OptionalAttackCost") {
		if sv.Source != ev.Obj {
			continue
		}
		if vc := sv.Params["ValidCard"]; vc != "" &&
			!effects.MatchesSpecFrom(e.G, vc, ev.Obj, o.Controller, sv.Source) {
			continue
		}
		exec := sv.Params["Trigger"]
		if exec == "" {
			continue
		}
		sa := grantedTriggerExecute(o, exec)
		if sa == nil {
			continue
		}
		key := triggerKey{Source: ev.Obj, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue // cascade bound: see maxTriggerFires.
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     ev.Obj,
			Controller: o.Controller,
			Idx:        -1,
			SA:         sa,
			Granted:    true,
			Execute:    exec,
			Ctx: effects.Ctx{
				Source:     ev.Obj,
				Controller: o.Controller,
			},
		})
	}
}

// checkFaceTriggers separates the read-only matching board from the live
// queue and firing limits. Both walks use deterministic seat/zone/slice order;
// the ordinary APNAP drain still asks each controller to order their triggers.
func (e *Engine) checkFaceTriggers(observer *Engine, ev events.Event, lki *state.Object,
	lkiPower, lkiToughness int32, lkiPTValid, split, leaving bool) {
	// phaseNotes collects the unresolvable Phase$ specs this walk encountered
	// (live walks only -- the leaves-the-battlefield look-back observer is a
	// scratch Engine that must never emit), each with the source that carries
	// them; they are emitted once, after the walk, so a Note emission's own
	// recursive checkTriggers can never interleave with the walk's matching.
	type phaseNote struct {
		id   state.ObjID
		spec string
	}
	var phaseNotes []phaseNote
	// Granted triggers inspect the same active-static list for every object
	// this event visits. Matching cannot emit or mutate continuous effects;
	// phase diagnostics emit only after the walk, so this snapshot is stable
	// for its full deterministic traversal.
	grantedStatics := observer.active()
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
		// trigger's Ctx both read it. Hoisted above the face-down gate too: the
		// cloaked ward walk below reads it as well.
		var objLKI *state.Object
		if lki != nil && lki.ID == ev.Obj {
			objLKI = lki
		}
		if e.faceDownPrintedHides(o) {
			// CR 708.8: a face-down permanent's printed triggers (and any
			// granted walk keyed to it) do not exist while it is face down --
			// a manifested Sultai Emissary that dies reveals itself as a card
			// in the graveyard and fires nothing (the leaves-battlefield
			// look-back observer reads the pre-move state, where it is still
			// face down; the live walk matches leaves-triggers only through
			// that observer or a TriggerZones the departed card no longer
			// occupies).
			// A CLOAKED face-down permanent is the one exception: its ward {2}
			// is part of the cloak status itself (CR 708.5's cloak variant),
			// not a printed ability -- the synthesized Ward trigger in
			// checkGrantedWardTriggers (built from the derived keyword list
			// layers.go appends for Cloaked objects) is what turns targeting
			// it into the pay-or-counter ask. Only the granted-ward walk
			// revives; the printed-face walk stays suppressed.
			if o.Cloaked && ev.Kind == events.TargetsChosen {
				e.checkGrantedWardTriggers(observer, id, o, f, ev, objLKI, lkiPower, lkiToughness, lkiPTValid)
			}
			return
		}
		// Ordinary cards need no face-walk setup when their printed triggers
		// cannot observe this event. An unlocked Room may still have an
		// eligible alternate face. Granted Ward is independent of both -- and
		// so is a static-grant's trigger (AddTrigger$): the granted walk below
		// runs on BOTH paths, like Ward and Dethrone do.
		if !o.Unlocked && !e.objectFaceMayTrigger(id, o.FaceIdx, f, ev.Kind) {
			if grantedKeywordTriggerEvent(ev.Kind) {
				switch ev.Kind {
				case events.TargetsChosen:
					e.checkGrantedWardTriggers(observer, id, o, f, ev, objLKI, lkiPower, lkiToughness, lkiPTValid)
				case events.DeclareAttackers:
					e.checkGrantedDethroneTriggers(observer, id, o, f, ev, objLKI)
				case events.DeclareBlockers:
					e.checkGrantedAfflictTriggers(id, o, f, ev)
				}
			}
			e.checkGrantedStaticTriggersUsing(observer, grantedStatics, id, o, ev, objLKI, lkiPower, lkiToughness, lkiPTValid)
			return
		}
		// Enchantment Rooms (rules/rooms.go): an UNLOCKED room's alternate
		// face is live too, so its triggers walk in the same scan. The face
		// index rides the triggerKey (Face field) so the alternate face's
		// fire-count and once-per-turn memory never share an entry with the
		// cast face's same-index trigger. roomTriggerFaces returns the faces
		// to walk, cast face first.
		faces, n := roomTriggerFaces(o, f)
		for _, fc := range faces[:n] {
			if o.Unlocked && !e.objectFaceMayTrigger(id, fc.faceIdx, fc.face, ev.Kind) {
				continue
			}
			for ti, t := range fc.face.Triggers {
				// LifeLostAll is evaluated once at the end of a simultaneous
				// life-loss batch. Do not queue it once per serialized Damage/
				// LifeChange event, and do not let the finishing pass re-check
				// unrelated trigger modes.
				if t.Mode == "LifeLostAll" && e.lifeLossBatchDepth > 0 && !e.finishingLifeLossBatch {
					continue
				}
				if e.finishingLifeLossBatch && t.Mode != "LifeLostAll" {
					continue
				}
				if !leaving {
					// Phase$ is a common Forge trigger gate, not a Mode$ Phase
					// parameter: ChangesZone, SpellCast, and every other supported
					// trigger mode may carry it. Report an unresolvable value once
					// per engine per spec, while triggerMatches rejects it on every
					// event. Keeping reporting outside the scratch look-back walk
					// means a Note is a real event, never an observer side effect.
					spec := t.Params["Phase"]
					if strings.TrimSpace(spec) != "" {
						if !e.parsedPhaseSpec(spec).valid {
							if e.phaseUnknownNoted == nil {
								e.phaseUnknownNoted = map[string]bool{}
							}
							if !e.phaseUnknownNoted[spec] {
								e.phaseUnknownNoted[spec] = true
								phaseNotes = append(phaseNotes, phaseNote{id: id, spec: spec})
							}
						}
					}
				}
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
				// Forge's Secondary$ True: a marked secondary is the second
				// half of one card text, and it does not fire when the same
				// event already fired its card's paired primary (the
				// two-halves idiom -- Sower of Discord's complementary
				// DamageDoneOnce pair, Wooden Stake's blocks-or-is-blocked-by
				// pair). A secondary whose primary did not fire for this event
				// still fires on its own. See secondaryYields for the pairing.
				if strings.EqualFold(t.Params["Secondary"], "True") &&
					e.secondaryYields(observer, fc.face, ti, t, id, ev, objLKI) {
					continue
				}
				if (t.Mode == "DamageDealtOnce" || t.Mode == "DamageDoneOnce") && ev.Amount <= 0 {
					continue
				}
				key := triggerKey{Source: id, Idx: ti, Face: fc.faceIdx}
				if e.triggerFireCount == nil {
					e.triggerFireCount = map[triggerKey]int32{}
				}
				if e.triggerFireCount[key] >= maxTriggerFires {
					continue // cascade bound: see maxTriggerFires.
				}
				if actionTriggerModes[t.Mode] && !e.triggerActivationLimitAllows(t, key) {
					continue // ActivationLimit$: already triggered enough this turn.
				}
				// ResolvedLimit$ ("Do this only once each turn."): scoped to EVERY
				// trigger mode, not just actionTriggerModes -- ChangesZone and
				// SpellCast, the two largest groups, are not in that set. The
				// count is keyed by the SOURCE object so a card's paired lines
				// share one limit; a malformed value denies (fail closed).
				if limit, present := resolvedLimitValue(t); present {
					f := e.triggerTurnResolved[id]
					if f.Turn != e.G.Turn {
						f = turnFires{Turn: e.G.Turn}
					}
					if int(f.N) >= limit {
						continue // ResolvedLimit$: already resolved enough this turn.
					}
				}
				if t.Mode == "DamageDealtOnce" || t.Mode == "DamageDoneOnce" {
					// The "Once" gate latches once per DAMAGE BATCH, not per turn
					// (CR 510.4; Forge PhaseHandler.dealAssignedDamage fires
					// triggerDamageDoneOnce once per damage step, and one
					// dealDamage call's damage per batch): a double striker's
					// bearer triggers Jitte twice (two damage steps), and two
					// separate damage events in one turn trigger an Enrage creature
					// twice -- while two blockers hitting back at the same creature
					// in ONE batch trigger it once, with the referent carrying the
					// batch's accumulated total. Within an open batch the entry
					// below IS the latch (a second matching event accumulates into
					// it); no batch open means every Damage event is its own batch,
					// so there is nothing to latch across and the trigger fires per
					// event with its own event amount already the batch total.
					// Non-positive amounts (the negative-amount Damage events the
					// cleanup/regeneration repair paths emit to clear marked
					// damage) are not damage and never latch or queue a Once
					// trigger.
					if ev.Amount > 0 {
						bk := damageBatchKey{triggerKey: key, dealt: t.Mode == "DamageDealtOnce"}
						if bk.dealt {
							// Combat identifies the actual attacker/blocker in damaging;
							// an effect batch's shared source is its published override
							// or its resolving stack object otherwise. A watcher can
							// match several sources in one batch, so never collapse
							// non-combat sources onto ObjID zero. Same priority order as
							// the ValidSource$ match above: an explicit override always
							// wins, e.damaging is combat-only, damageSource is the
							// non-combat fallback.
							bk.obj = e.dmgSrcOverride
							if bk.obj == 0 {
								if e.combatDamaging {
									bk.obj = e.damaging
								} else {
									bk.obj = e.damageSource()
								}
							}
						} else if ev.Obj != 0 {
							bk.obj = ev.Obj
						} else {
							bk.player = ev.Player
						}
						if e.damageBatchOpen {
							if e.damageBatchIdx == nil {
								e.damageBatchIdx = map[damageBatchKey]int{}
							}
							if entIdx, ok := e.damageBatchIdx[bk]; ok {
								e.damageBatchLog[entIdx].amount += ev.Amount
								continue // already queued once for this batch and referent.
							}
							e.damageBatchIdx[bk] = len(e.damageBatchLog)
							e.damageBatchLog = append(e.damageBatchLog, damageBatchEntry{
								key: bk, idx: len(e.pendingTriggers), amount: ev.Amount,
							})
							// Fall through: the trigger queues now, at the same point
							// in the stream it queued at before this gate was
							// batch-scoped; closeDamageBatch patches its referent
							// amount to the batch total.
						}
					}
				}
				e.triggerFireCount[key]++
				if t.Effect == nil {
					// Execute$ named an SVar this face never defined (or one
					// that failed to parse): the trigger matched, but there is
					// nothing to run.
					continue
				}
				// CR 603.3a/603.10a: a leaves-the-battlefield ability's source
				// is controlled by whoever controlled it as it left, not by the
				// owner the move has since reset it to (a stolen creature's own
				// dies trigger belongs to the player who stole it).
				controller := o.Controller
				if objLKI != nil && id == ev.Obj && leftBattlefield(ev) {
					controller = objLKI.Controller
				}
				// The non-active face of an unlocked Room must be minted through
				// the delayed-shape push: TriggerPush re-derives an ability from
				// the object's active Face(), while the delayed push resolves the
				// other face's Execute$ SVar directly. This is independent of
				// whether CR 309.4b cast face 0 or face 1.
				alt := !fc.active
				pt := pendingTrigger{
					Source:     id,
					Controller: controller,
					Idx:        ti,
					SA:         t.Effect,
					Ctx: effects.Ctx{
						Source:         id,
						Controller:     controller,
						Remembered:     triggerRemembered(ev, id),
						Captured:       triggerRemembered(ev, id),
						LKI:            objLKI,
						LKIPower:       lkiPower,
						LKIToughness:   lkiToughness,
						LKIPTValid:     objLKI != nil && lkiPTValid,
						TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
					},
				}
				if alt {
					pt.Delayed = true
					pt.DelayedID = ^uint32(0)
					pt.Execute = t.Params["Execute"]
				}
				e.pendingTriggers = append(e.pendingTriggers, pt)
				// stat:Panharmonicon (CR 702.109): "If a triggered ability of a
				// ... permanent you control triggers, that ability triggers an
				// additional time." Each battlefield Panharmonicon-shaped static
				// whose ValidCard$ matches the triggering object appends ONE extra
				// copy of this trigger immediately after the original, in scan
				// order -- deterministic, and the extra copy is an ordinary queue
				// entry that resolves like any other (the doubling does not fire on
				// the copy again: the copy is not an event). Panharmonicon's own
				// trigger is excluded by the spec's Other predicate, which is
				// relative to the Panharmonicon permanent itself.
				for k := 0; k < e.panharmoniconEchoes(observer.G, id, ev); k++ {
					e.pendingTriggers = append(e.pendingTriggers, pt)
				}
			}
		}
		e.checkGrantedStaticTriggersUsing(observer, grantedStatics, id, o, ev, objLKI, lkiPower, lkiToughness, lkiPTValid)
		// A granted Afflict must fire even when the object's own printed
		// triggers are live for this event (a Zombie with its own become-blocked
		// trigger carrying the Monarch's grant) -- the early-return path above
		// reaches this object through checkGrantedAfflictTriggers's own call.
		e.checkGrantedAfflictTriggers(id, o, f, ev)
	})
	for _, n := range phaseNotes {
		e.emit(events.Event{Kind: events.Note, Obj: n.id,
			Text: "Phase$ " + n.spec + " names no engine step; the trigger never fires"})
	}
}

// triggerFace is one face's trigger walk: its printed index keys fire-count
// memory, and active says whether TriggerPush can re-derive it through the
// object's current Face() (the other unlocked Room face must use a delayed
// shape because Apply cannot select it).
type triggerFace struct {
	face    *cards.Face
	faceIdx uint8
	active  bool
}

// roomTriggerFaces returns the faces whose Triggers a scan walks for object
// o: its cast face always, plus the other face once unlocked (CR 309.6).
// FaceIdx need not be zero: CR 309.4b permits casting either Room door.
func roomTriggerFaces(o *state.Object, active *cards.Face) ([2]triggerFace, int) {
	// At most two faces, returned by value so the ordinary single-face walk
	// never allocates a backing slice per object per event.
	out := [2]triggerFace{{face: active, faceIdx: o.FaceIdx, active: true}}
	if o.Unlocked && isRoom(o) && len(o.Card.Faces) == 2 && int(o.FaceIdx) < len(o.Card.Faces) {
		other := uint8(1 - int(o.FaceIdx))
		out[1] = triggerFace{face: o.Card.Faces[other], faceIdx: other}
		return out, 2
	}
	return out, 1
}

// openDamageBatch opens a damage batch: the Damage events emitted until the
// matching closeDamageBatch are one simultaneous batch for the
// DamageDealtOnce/DamageDoneOnce latch (CR 510.4; Forge dealAssignedDamage).
// Reentrant brackets belong to the same simultaneous batch: depth makes an
// inner close consume only its own begin, so it cannot close the outer batch
// early.
func (e *Engine) openDamageBatch() {
	if e.damageBatchDepth == 0 {
		e.damageBatchOpen = true
		e.damageBatchIdx = nil
		e.damageBatchLog = nil
	}
	e.damageBatchDepth++
}

// closeDamageBatch closes the open damage batch: every entry's queued trigger
// gets its referent patched to the batch's accumulated total (two blockers
// hitting one Enrage creature is ONE trigger whose amount is the sum), then
// the batch bookkeeping is dropped. The Once latch lives entirely inside the
// open batch -- entries are the latch, and closing clears them -- so nothing
// persists between batches and no per-turn latch remains. The pendingTriggers
// index recorded at queue time is re-checked against the trigger it was
// recorded for before patching; a mismatch (impossible today -- the queue is
// append-only while a batch is open, and nothing drains mid-batch) falls back
// to the first event's amount rather than patching a stranger.
func (e *Engine) closeDamageBatch() {
	if e.damageBatchDepth == 0 {
		return
	}
	e.damageBatchDepth--
	if e.damageBatchDepth != 0 {
		return
	}
	e.damageBatchOpen = false
	for _, ent := range e.damageBatchLog {
		if ent.idx >= len(e.pendingTriggers) {
			continue
		}
		pt := &e.pendingTriggers[ent.idx]
		if pt.Source != ent.key.Source || pt.Idx != ent.key.Idx {
			continue
		}
		pt.Ctx.TriggerContext.TriggerAmount = ent.amount
	}
	e.damageBatchIdx = nil
	e.damageBatchLog = nil
}

// BeginDamageBatch/EndDamageBatch are effects.Host's damage-batch bracket
// (effects/damage.go calls them around each dealDamage-style call); they are
// openDamageBatch/closeDamageBatch on the engine. See there.
func (e *Engine) BeginDamageBatch() { e.openDamageBatch() }
func (e *Engine) EndDamageBatch()   { e.closeDamageBatch() }

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
	// The scanner has already run its diagnostic/batch gates. Reject an
	// impossible event before consulting dynamic zone and phase predicates.
	if !triggerModeEvents(t.Mode).allows(ev.Kind) {
		return false
	}
	if !e.zoneGate(t, source, ev) || !e.phaseGate(t) {
		return false
	}
	var matched bool
	switch t.Mode {
	case "ChangesZone", "ChangesZoneAll":
		// ChangesZoneAll shares the per-object matcher (batch-of-one: see the
		// ValidCards plural key there); the once-per-group semantics Forge's
		// name promises for a multi-object operation is the recorded
		// ActivationLimit-driven narrowing for the lines that carry one and an
		// open approximation for the rest.
		matched = e.zoneChangeMatches(t, source, ev, lki)
	case "SpellCast":
		matched = e.spellCastMatches(t, source, ev)
	case "SpellCastOrCopy":
		// The magecraft family ("Whenever you cast or copy an instant or
		// sorcery spell, ..."): the cast half is the ordinary SpellCast
		// evaluation on the PutOnStack event; the copy half delegates a
		// StackCopy event to spellCopyMatches. A copy never re-enters the
		// stack as a PutOnStack -- effects/copy.go emits events.StackCopy
		// naming the original -- so spellCastMatches' entering-the-stack guard
		// would keep the copy half dead if the whole mode fell through to it.
		if ev.Kind == events.StackCopy {
			matched = e.spellCopyMatches(t, source, ev)
		} else {
			matched = e.spellCastMatches(t, source, ev)
		}
	case "SpellCopy":
		// The copy-only mode ("Whenever you copy a spell, ..."): plain casts
		// are not copies, so a PutOnStack event must not fire it.
		matched = e.spellCopyMatches(t, source, ev)
	case "AbilityCast", "SpellAbilityCast":
		matched = e.abilityCastMatches(t, source, ev)
	case "Attacks":
		matched = e.attacksMatches(t, source, ev)
	case "AttackersDeclared", "AttackersDeclaredOneTarget":
		matched = e.attackersDeclaredOneTargetMatches(t, source, ev)
	case "Cycled":
		matched = e.cycledMatches(t, source, ev, lki)
	case "Explores":
		matched = e.exploresMatches(t, source, ev, lki)
	case "RingTemptsYou":
		matched = e.ringTemptsMatches(t, source, ev)
	case "CounterAdded":
		matched = e.counterAddedMatches(t, source, ev, lki)
	case "CounterRemoved":
		matched = e.counterRemovedMatches(t, source, ev, lki)
	case "Attached":
		matched = e.attachedMatches(t, source, ev)
	case "Exerted":
		matched = e.exertedMatches(t, source, ev)
	case "TokenCreated", "TokenCreatedOnce":
		matched = e.tokenCreatedMatches(t, source, ev)
	case "Sacrificed":
		matched = e.sacrificedMatches(t, source, ev, lki)
	case "Discarded":
		matched = e.discardedMatches(t, source, ev)
	case "CommitCrime":
		matched = e.commitCrimeMatches(t, source, ev)
	case "Taps":
		matched = e.tapsMatches(t, source, ev, false)
	case "TapsForMana":
		matched = e.tapsMatches(t, source, ev, true)
	case "DamageDone", "DamageDealtOnce", "DamageDoneOnce":
		matched = e.damageMatches(t, source, ev)
	case "DamagePreventedOnce":
		matched = e.damagePreventedMatches(t, source, ev)
	case "FlippedCoin":
		matched = e.flippedCoinMatches(t, source, ev)
	case "Drawn":
		matched = e.drawnMatches(t, source, ev)
	case "LifeLost", "LifeLostAll":
		matched = e.lifeLostMatches(t, source, ev)
	case "BecomesTarget":
		matched = e.becomesTargetMatches(t, source, ev)
	case "LandPlayed":
		matched = e.landPlayedMatches(t, source, ev)
	case "Phase":
		matched = e.phaseMatches(t, source, ev)
		// kw:Echo (CR 702.35a) rides the Echo$ True marker on its generated
		// keyword trigger the way Annihilator$ rides its own: the intervening-if
		// must suppress the trigger BEFORE it stacks (a stacked-but-owed-nothing
		// echo is an observable divergence). The gate reads the object's
		// control-acquisition tuple (rules/echo.go) against the controller's
		// most recent upkeep.
		if matched && t.Params["Echo"] == "True" {
			matched = e.echoGateHolds(source)
		}
	case "Always":
		// CR 603.8 state trigger: the event under test is irrelevant; the
		// trigger fires when its condition holds (see triggerConditionHolds)
		// and no instance is outstanding (the checkTriggers latch above).
		matched = true
	}
	if !matched {
		return false
	}
	// FirstCombat$ True -- the "if it's the first combat phase of the turn"
	// trigger gate (8 corpus T: lines: hexplate_wallbreaker, genji_glove,
	// finest_hour, balthier_and_fran, raph_leo_sibling_rivals on Mode$ Attacks,
	// karlach_fury_of_avernus on Mode$ AttackersDeclared,
	// zariel_archduke_of_avernus and swinging_ship on Mode$ Phase) restricts
	// the trigger to the FIRST combat phase begun this turn, so a trigger an
	// extra combat (DB$ AddPhase) creates must not re-fire. The count is the
	// event-folded state.Game.CombatsThisTurn (one increment per BeginCombat
	// entry, reset at TurnChange), evaluated on the fold of the matching
	// event: during the first combat it is 1 (its BeginCombat already
	// folded), and every extra combat's BeginCombat has raised it to 2 -- so
	// the test is count == 1, never 0. This is a SHARED gate in
	// triggerMatches rather than a per-mode one because the key rides three
	// different modes and means the same thing on every one; it is distinct
	// from the Execute-side ConditionFirstCombat$ gate
	// (effects/conditions.go, Raiyuu), which suppresses the BODY, not the
	// trigger.
	if strings.EqualFold(strings.TrimSpace(t.Params["FirstCombat"]), "True") && e.G.CombatsThisTurn != 1 {
		return false
	}
	// PlayerTurn$ True: only during the turn of the source's controller
	// (Forge Trigger.requirementsCheck), scoped to actionTriggerModes.
	if actionTriggerModes[t.Mode] && strings.EqualFold(t.Params["PlayerTurn"], "True") &&
		e.G.Active != e.controllerOf(source) {
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

// secondaryYields implements Forge's Secondary$ True (TriggerHandler,
// Trigger.isSecondary): the marked trigger yields when the SAME event would
// also fire its card's paired primary, so one card text never becomes two
// triggers. The pairing is strictly by shared Execute$ SVar: the corpus's
// paired halves share the one SVar their two conditions resolve into (the
// "enters or attacks" family -- Grave Titan, Sun Titan, Tome of Legends,
// Kindred Discovery, Zoraline's "enters or attacks" half -- plus the
// Eminence family's command-zone/battlefield halves, which the zone gate
// keeps mutually exclusive anyway). A secondary whose paired primary did not
// fire for this event still fires on its own -- Grave Titan's Attacks half
// fires for the attack although its ETB half did not.
//
// A Mode-equality fallback was tried here and REMOVED (r2 review): pairing
// any same-Mode sibling suppresses independent co-firing abilities, not just
// complementary halves. Zoraline, Cosmos Caller carries two Attacks triggers
// on one face -- her printed "Whenever a Bat you control attacks, gain 1
// life" and the Secondary$-marked "enters or attacks" half -- and for one
// DeclareAttackers event both match, so the fallback lost her printed
// trigger (Vengeful Ancestor and Ashling, Rimebound were the suspected
// trace-level victims of the same shape). The corpus-measured picture for
// same-Mode pairs with distinct Execute$ SVars is exactly the two shapes the
// fallback conflated: the genuinely-complementary halves it was built for
// (Wooden Stake's blocks-or-is-blocked-by pair, Sower of Discord's two
// DamageDoneOnce halves, the Clashed Won$ True/False pairs) have disjoint
// Valid halves and never double-fire without a yield, while the pairs that
// DO co-fire on one event -- Zoraline's two Attacks triggers, Sephiroth's
// printed attack trigger beside its marked "enters or attacks" half,
// Ashling, Rimebound's Main1 mana burst beside its transform offer -- are
// independent card texts that SHOULD both fire. No distinct-Execute pair
// needs a yield; none gets one.
func (e *Engine) secondaryYields(observer *Engine, face *cards.Face, ti int, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	for j, sib := range face.Triggers {
		if j == ti || strings.EqualFold(sib.Params["Secondary"], "True") {
			continue
		}
		if sib.Params["Execute"] != t.Params["Execute"] {
			continue
		}
		if observer.triggerMatches(sib, source, ev, lki) {
			return true
		}
	}
	return false
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
		// Forge's ActiveZones$ is the trigger-side spelling of the same gate
		// (the replacement side already reads the key:
		// rules/replacement.go's zone gate). Sower of Discord's two
		// DamageDoneOnce halves declare ActiveZones$ Battlefield; an explicit
		// ActiveZones$ is authoritative exactly like an explicit
		// TriggerZones$, so the two special cases below keep treating it as
		// declared.
		spec = t.Params["ActiveZones"]
	}
	if spec == "" && ev.Kind == events.PutOnStack && source == ev.Obj && t.Mode == "SpellCast" {
		// CR 601.2i: the spell's OWN cast trigger fires while the source is
		// the spell sitting on the stack -- exactly the event being walked.
		// The battlefield default would gate it out (the source is in ZStack,
		// and the PutOnStack look-back zone below is the zone it came FROM,
		// the hand), so every bare "When you cast this spell" script --
		// Hydroid Krasis, Genesis Hydra, Ulamog, World Breaker -- would
		// never fire at all. An EXPLICIT TriggerZones$ stays authoritative:
		// a script naming one knows where its trigger lives.
		return true
	}
	if spec == "" {
		// "When you discard this card" (Orvar, Bartered Cow, Titanbones: 14
		// of the corpus's Mode$ Discarded lines) declares no TriggerZones$,
		// and the only zone a card is discarded from is its owner's hand (CR
		// 701.9a). Forge applies no zone restriction to a trigger without
		// TriggerZones$; the battlefield default below would leave such a
		// trigger unable to fire at all, so the card's own discard admits it
		// wherever the discard (or a replacement redirecting it) put it.
		if t.Mode == "Discarded" && source == ev.Obj && events.IsDiscard(ev) {
			return true
		}
		// The cycled card itself is the moved card (ValidCard$ Card.Self):
		// its own cycle-trigger must fire from wherever the cost discard (or a
		// replacement redirecting it) put it, the same courtesy the Discarded
		// case above extends.
		if t.Mode == "Cycled" && source == ev.Obj && events.IsDiscard(ev) {
			return true
		}
		// A card's own hand->exile move (Lupine Harbingers' "note the number
		// of turns you've begun since it was foretold" exile trigger -- the
		// corpus's ONE ChangesZone self-trigger with no TriggerZones$ whose
		// destination is not the battlefield, measured over the corpus pin):
		// the only zone the trigger can observe the move from is the hand the
		// card sits in. Forge applies no zone restriction to a trigger
		// without TriggerZones$; the battlefield default would leave such a
		// trigger unable to fire at all, so the card's own hand-origin move
		// admits it, the same courtesy the Discarded and Cycled cases above
		// extend. (Self-moves whose DESTINATION is the battlefield need no
		// admission: the post-move zone check below already sees them.)
		if t.Mode == "ChangesZone" && source == ev.Obj && ev.Kind == events.MoveZone &&
			ev.From == state.ZHand {
			return true
		}
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
	// ExcludedOrigins$ ("Name Sticker" Goblin's "enters from anywhere other
	// than a graveyard or exile"): a comma-separated list of zones the move
	// must NOT originate in. Absent means unrestricted, exactly as before.
	if excl, ok := t.Params["ExcludedOrigins"]; ok {
		for _, z := range strings.Split(excl, ",") {
			if zz := strings.TrimSpace(z); zz != "" && effects.ParseZone(zz) == ev.From {
				return false
			}
		}
	}
	if d, ok := t.Params["Destination"]; ok && d != "Any" && effects.ParseZone(d) != ev.To {
		return false
	}
	// ValidCards$ is the PLURAL key the ChangesZoneAll corpus uses (124 of
	// its 126 lines); ValidCard$ is the singular key ChangesZone uses. One
	// matcher serves both modes, so read the plural first and fall back.
	v, hasSpec := t.Params["ValidCards"]
	if !hasSpec {
		v, hasSpec = t.Params["ValidCard"]
	}
	if hasSpec {
		// The trigger's own source moving (source == ev.Obj) with an LKI
		// snapshot available is a dying card asserting a property about
		// itself, e.g. Undying's counters_EQ0_P1P1: read it against the LKI
		// (what it was the moment before the move reset it), not the live
		// object already in the destination zone.
		// A permanent that LEFT the battlefield is likewise matched as it
		// last existed there (CR 603.10a) -- in particular its controller:
		// "a creature you control dies" must see a stolen creature as the
		// taker's, though the move has already handed it back to its owner.
		// ctrl is the trigger source's controller at that moment too, which
		// for the departed source itself is its LKI controller.
		ctrl := e.controllerOf(source)
		if source == ev.Obj && lki != nil && leftBattlefield(ev) {
			ctrl = lki.Controller
		}
		// The bare wasCastFromYourHandByYou qualifier (the "if you cast it
		// from your hand" ETB family) is split out and evaluated against the
		// log here, where the Engine is in scope; the remainder matches as
		// before (task castprov1).
		if ev.Obj != 0 && lki != nil && (source == ev.Obj || leftBattlefield(ev)) {
			spec, ok := e.castProvenanceAdmits(v, lki.ID, ctrl)
			if !ok || !effects.MatchesObjectCtx(e.G, spec, lki, e.specCtx(source, ctrl)) {
				return false
			}
		} else {
			spec, ok := e.castProvenanceAdmits(v, ev.Obj, e.controllerOf(source))
			if !ok || !effects.MatchesSpecCtx(e.G, spec, ev.Obj, e.specCtx(source, e.controllerOf(source))) {
				return false
			}
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
// ValidActivatingPlayer$ against a PutOnStack event, plus the two cast-
// condition clauses the measured cards carry -- ActivatorThisTurnCast$
// (The Lord of Pain, Vial Smasher the Fierce: <OP><N> over the spells the
// ACTIVATOR has cast this turn, the current cast included -- its PutOnStack
// is already in the log when the deferred trigger fires) and ValidSA$
// (Roiling Vortex: the Spell.ManaSpent <OP><value> comparison family).
func (e *Engine) spellCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	return e.spellCastEval(t, source, ev)
}

// spellCopyMatches is the copy half of the spell-cast family: Mode$
// SpellCastOrCopy delegates a StackCopy event here, and Mode$ SpellCopy
// ("Whenever you copy a spell, ...", the_parnesse_the_subtle_brush shape)
// is its only mode. Copies do not re-enter the stack as a PutOnStack --
// effects/copy.go emits events.StackCopy naming the ORIGINAL spell (ev.Obj,
// still on the stack; events.Apply's StackCopy case rejects anything else)
// with ev.Player the copy's controller -- and the copy object shares the
// original's card, face and CastFlags, so the shared evaluation below reads
// the copied spell identically. A plain SpellCast trigger stays silent here
// (a copy is not a cast) and a SpellCastOrCopy/SpellCopy trigger does not
// fire on the cast of the spell itself -- that half of the division is
// spellCastMatches'.
func (e *Engine) spellCopyMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StackCopy {
		return false
	}
	return e.spellCastEval(t, source, ev)
}

// spellCastEval is the spell evaluation spellCastMatches and
// spellCopyMatches share, minus the entering-the-stack guard each mode owns:
// ValidCard$ (through the trigger-side cast alternatives and the
// cast-provenance qualifiers), ValidActivatingPlayer$, the
// ActivatorThisTurnCast[Each] counts, ValidSA$ and HasXManaCost$ -- read off
// ev.Obj (the spell being cast, or the original being copied) and ev.Player
// (the cast's or the copy's controller) exactly alike.
func (e *Engine) spellCastEval(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	// Casting a spell means an actual card entering the stack (a copy
	// evaluates the ORIGINAL, which Apply's StackCopy case guarantees is on
	// the stack). This build also uses PutOnStack-shaped Move()s for nothing else today (triggered
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
	// The ValidCard alternatives (post trigger-side self-cast exclusion) are
	// computed ONCE: the cast's own match below and the
	// ActivatorThisTurnCastEach$ tally must read the same surviving set, so
	// the two can never disagree (the Each arm below reads only the alts this
	// block computed).
	var castAlts []triggerCastAlt
	if v, ok := t.Params["ValidCard"]; ok {
		alts, ok2 := e.triggerCastAlternatives(v, source, ev.Obj)
		if !ok2 {
			return false
		}
		spec := ""
		for i, alt := range alts {
			if i > 0 {
				spec += ","
			}
			spec += alt.spec
		}
		// The bare wasCastFromYourHandByYou / wasCastByYou qualifiers (the
		// cast-provenance families, tasks castprov1/castprov2) are split out
		// and evaluated against the log here; the remainder matches as before.
		spec, ok3 := e.castProvenanceAdmits(spec, ev.Obj, ctrl)
		if !ok3 || !effects.MatchesSpecCtx(e.G, spec, ev.Obj, e.specCtx(source, ctrl)) {
			return false
		}
		castAlts = alts
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	if v, ok := t.Params["ActivatorThisTurnCast"]; ok {
		if !compareIntCount(int32(e.spellsCastThisTurn(ev.Player)), v) {
			return false
		}
	}
	if v, ok := t.Params["ActivatorThisTurnCastEach"]; ok {
		// The PER-ALTERNATIVE first-cast read (task castprov2, Alania,
		// Divergent Storm — the corpus's one carrier): the trigger fires when
		// the activator's cast is the FIRST this turn of at least ONE
		// ValidCard$ alternative — a disjunction of firsts, so casting an
		// instant and then a sorcery fires on the sorcery. The tally is the
		// activator's casts this turn matching that alternative, the current
		// cast INCLUDED (it is already in the log when the deferred trigger
		// fires; EQ1 means this cast is the first).
		//
		// Only alternatives the CURRENT CAST MATCHES are evaluated (round-2
		// review): the oracle reads "if it's the first instant ... you've cast
		// this turn" — the firstness must attach to the spell being cast, so
		// a second instant after instant→sorcery does NOT fire on the
		// sorcery alternative's earlier tally (that sorcery was the first
		// sorcery, but this cast is not one). An alternative whose tally is
		// met while the current cast matches a DIFFERENT alternative is
		// skipped; without the guard the trigger false-fires on any
		// two-cast-then-repeat turn. castAlts comes from the same surviving
		// set the cast's own match above used, so the two cannot disagree.
		//
		// A trigger carrying the Each param with NO ValidCard$ has no
		// alternatives to attach the firstness to: UNSUPPORTED, fail closed
		// (castAlts stays nil; the corpus's one carrier, Alania, always has
		// a ValidCard$).
		fired := false
		for _, alt := range castAlts {
			if !effects.MatchesSpecCtx(e.G, alt.spec, ev.Obj, e.specCtx(source, ctrl)) {
				continue
			}
			if compareIntCount(int32(e.spellsCastThisTurnByMatching(ev.Player, alt.spec, alt.exclSelf, source)), v) {
				fired = true
				break
			}
		}
		if !fired {
			return false
		}
	}
	if v, ok := t.Params["ValidSA"]; ok {
		if !e.validSAMatches(source, ev, ctrl, v) {
			return false
		}
	}
	if !hasXManaCostGate(t.Params, obj.Face().ManaCost, nil) {
		return false
	}
	return true
}

// hasXManaCostGate implements HasXManaCost$ True on cast/activation trigger
// modes (Mode$ SpellCast and Mode$ AbilityCast/SpellAbilityCast): "whenever
// you cast a permanent spell with a mana cost that contains {X}" / "...or
// activate an ability ... if that ability's activation cost contains {X}"
// (Unbound Flourishing, Glava Five-Advents Mage, Brass Infiniscope, Magus
// Lucea Kane). The gate reads the PRINTED cost -- the spell's ManaCost face
// field, or the ability's Cost$ param -- through ParseCost, which counts only
// the mana {X} tokens (c.X); a non-mana component token such as
// SubCounter<X/CHARGE> or PayEnergy<X> is matched by its own grammar and does
// NOT count, which is exactly the card text's "mana cost/activation cost
// contains {X}". For an activation the printed Cost$ still carries the X at
// AbilityPush time (the announced value folds into the provisional payment
// cost, rules/cast.go's pc.cost.WithX, never into the SA params), so the
// gate is about the printed shape, never the announced value.
//
// A param present with a value other than True FAILS CLOSED (the trigger
// stays silent), per the repo's unreadable-condition convention; a param
// absent leaves the trigger's behaviour unchanged. ab is the activated
// ability on the AbilityCast arm (nil on the SpellCast arm, where faceCost is
// the spell's printed face cost); a nil ab with an empty faceCost fails
// closed rather than firing wide.
func hasXManaCostGate(params map[string]string, faceCost string, ab *cards.SA) bool {
	v, ok := params["HasXManaCost"]
	if !ok {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(v), "True") {
		return false
	}
	if ab != nil {
		return ParseCost(ab.Params["Cost"]).X > 0
	}
	return ParseCost(faceCost).X > 0
}

// compareIntCount evaluates Forge's <OP><N> comparison grammar (EQ1, GT1,
// EQ0, ...) against n. A value that is not a literal comparison (an X, a
// bare word, an unknown operator) fails closed: a trigger condition the
// engine cannot evaluate must stay silent, never fire wide.
func compareIntCount(n int32, expr string) bool {
	expr = strings.TrimSpace(expr)
	for _, cand := range []struct {
		op string
		fn func(a, b int32) bool
	}{{"EQ", func(a, b int32) bool { return a == b }},
		{"NE", func(a, b int32) bool { return a != b }},
		{"GE", func(a, b int32) bool { return a >= b }},
		{"LE", func(a, b int32) bool { return a <= b }},
		{"GT", func(a, b int32) bool { return a > b }},
		{"LT", func(a, b int32) bool { return a < b }}} {
		if rest := strings.TrimPrefix(expr, cand.op); rest != expr {
			v, err := strconv.Atoi(strings.TrimSpace(rest))
			if err != nil {
				return false
			}
			return cand.fn(n, int32(v))
		}
	}
	return false
}

// manaSpentForCast sums the mana the player spent casting the spell ev put
// on the stack: every negative ManaAdd for that player since the spell's
// own PutOnStack. Between CR 601.2a's push and the deferred cast trigger
// the only mana leaving a pool is this cast's payment (a mana window adds
// mana, it never spends), and the scan reads the log, so a replay derives
// the identical number.
func (e *Engine) manaSpentForCast(p state.PlayerID, id state.ObjID) int32 {
	var spent int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == id && ev.Player == p {
			return spent
		}
		if ev.Kind == events.ManaAdd && ev.Player == p && ev.Amount < 0 {
			spent += -ev.Amount
		}
	}
	return spent
}

// validSAMatches evaluates a SpellCast trigger's ValidSA$ clause. The
// measured grammar is the mana comparison family, "Spell.ManaSpent <OP><N>"
// (Roiling Vortex's EQ0 -- no mana was spent to cast that spell; Raggadragga
// and the emperor's GE7/EQ0 shapes read the same head): the value is the
// mana the ACTIVATOR paid for the cast, so GTX/other dynamic values fail
// closed. A clause naming an unmodelled property (ManaSpentBy, MayPlaySource,
// Self, YouCtrl) or carrying no comparison is evaluated as a plain spec
// filter over the cast spell when it is a single field, and fails closed
// otherwise.
func (e *Engine) validSAMatches(source state.ObjID, ev events.Event, ctrl state.PlayerID, clause string) bool {
	fields := strings.Fields(strings.TrimSpace(clause))
	switch len(fields) {
	case 1:
		return effects.MatchesSpecCtx(e.G, fields[0], ev.Obj, e.specCtx(source, ctrl))
	case 2:
		if fields[0] == "Spell.ManaSpent" {
			return compareIntCount(e.manaSpentForCast(ev.Player, ev.Obj), fields[1])
		}
		return false
	}
	return false
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
	if strings.EqualFold(strings.TrimSpace(t.Params["Myriad"]), "True") {
		// Myriad$ True is the myriad keyword expansion's own marker
		// (cards/keywords.go addKeywordTrigger): the per-other-opponent token
		// copies are the DB$ Myriad body, so the marker requires the
		// trigger's Execute sub to resolve to exactly that body -- a
		// mismatched or unresolvable expansion must not fire.
		src := e.G.Obj(source)
		if src == nil || src.Face() == nil {
			return false
		}
		sa := cards.ResolveSVar(src.Face().SVars, t.Params["Execute"])
		if sa == nil || sa.API != "Myriad" {
			return false
		}
	}
	if v, ok := t.Params["Alone"]; ok && strings.EqualFold(v, "True") && len(ev.IDs) != 1 {
		return false
	}
	// Dethrone (CR 702.105) fires only when the attacked player has the
	// greatest life total (tied is enough) among ALL players. Comparing only
	// the attacker and its defender is wrong in multiplayer: a third player
	// with more life prevents the trigger even though it was not attacked.
	if v, ok := t.Params["Dethrone"]; ok && strings.EqualFold(v, "True") {
		if int(ev.Player) >= len(e.G.Players) || e.G.Players[ev.Player].Lost {
			return false
		}
		life := e.G.Players[ev.Player].Life
		for i := range e.G.Players {
			if !e.G.Players[i].Lost && e.G.Players[i].Life > life {
				return false
			}
		}
	}
	// FirstAttack$ True (Aurelia the Warleader, Godo Bandit Warlord, Scourge
	// of the Throne, Fear of Missing Out -- the four corpus carriers, all the
	// plain True spelling) gates the trigger to the attacker's FIRST attack
	// this turn (CR 603.2e's "for the first time each turn"). The count is
	// event-folded state (Object.AttacksThisTurn, reset at TurnChange -- an
	// extra combat inside the same turn does not reset it), and trigger
	// matching runs on the FOLDED event, so the test is count == 1, never 0.
	// A non-first attacker must not veto the match either: an event may name
	// several attackers and another one may still be first.
	spec, ok := t.Params["ValidCard"]
	if !ok {
		for _, id := range ev.IDs {
			if id == source {
				return e.firstAttackOK(t, id)
			}
		}
		return false
	}
	ctrl := e.controllerOf(source)
	for _, id := range ev.IDs {
		if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, ctrl)) && e.firstAttackOK(t, id) {
			return true
		}
	}
	return false
}

// firstAttackOK reports whether the matched attacker passes the trigger's
// FirstAttack$ gate (nil-safe: a trigger without the param always passes).
func (e *Engine) firstAttackOK(t cards.Trigger, id state.ObjID) bool {
	if v, ok := t.Params["FirstAttack"]; !ok || !strings.EqualFold(strings.TrimSpace(v), "True") {
		return true
	}
	o := e.G.Obj(id)
	return o != nil && o.AttacksThisTurn == 1
}

// attackersDeclaredOneTargetMatches implements the "whenever [one or more]
// creatures attack a player" trigger (Forge Mode$ AttackersDeclaredOneTarget)
// and, routed to the same matcher, the batch "whenever you attack" trigger
// (Forge Mode$ AttackersDeclared) -- both read the same per-defender
// DeclareAttackers event the engine emits, and both admit exactly the same
// trigger-level parameters (AttackingPlayer$, AttackedTarget$,
// ValidAttackers$, ValidAttackersAmount$). handleAttackers emits one
// DeclareAttackers event per defender, so this fires once for each attacked
// player, not once for every attacker in that group. A batch AttackersDeclared
// trigger therefore fires once per attacked player on a split attack (one
// declare step, several events) -- the known limitation recorded in
// AGENTS.md's approximations table, not silently.
func (e *Engine) attackersDeclaredOneTargetMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.DeclareAttackers || len(ev.IDs) == 0 {
		return false
	}
	ctrl := e.controllerOf(source)
	attacker := e.controllerOf(ev.IDs[0])
	if v := t.Params["AttackingPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, attacker, ctrl) {
		return false
	}
	if v := t.Params["AttackedTarget"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
		return false
	}
	matches := 0
	for _, id := range ev.IDs {
		if v := t.Params["ValidAttackers"]; v == "" || effects.MatchesSpecCtx(e.G, v, id, e.specCtx(source, ctrl)) {
			matches++
		}
	}
	if matches == 0 {
		return false
	}
	if v := t.Params["ValidAttackersAmount"]; v != "" && !comparePresent(matches, v) {
		return false
	}
	return true
}

// cycledMatches implements the "when you cycle [this card]" trigger (CR
// 702.78d's cycling trigger, Forge Mode$ Cycled -- Dismantling Wave, 77
// corpus files). The engine's cycle activation discards the card as its
// cost, so the causing event is that cost discard (events.DiscardCost's
// canonical hand-to-graveyard move), and the moved card's PRINTED Cycling
// keyword is what makes a cost discard a cycle: an ordinary discard (a
// Wheel effect) is not one, and neither is a cycling card discarded as the
// cost of a different card's ability. The printed-keyword limit is the same
// one the granted-keyword Dethrone check documents: a card whose cycling is
// granted in a layer rather than printed never matches. ValidCard$ is
// matched against the moved card's LKI -- the card is already in its
// destination zone when triggers are checked, exactly like Sacrificed.
// The cycler is the moved card's controller: a card in a hand is controlled
// by its owner, and DiscardCost carries no player field to read instead.
func (e *Engine) cycledMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if !events.IsDiscardCost(ev) {
		return false
	}
	o := lki
	if o == nil {
		o = e.G.Obj(ev.Obj)
	}
	if o == nil || o.Face() == nil || !o.Face().HasKeyword("Cycling") {
		return false
	}
	return e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller)
}

// exploresMatches implements the "Whenever a creature you control explores
// ..." trigger family (Forge Mode$ Explores, task explore1 — Merfolk
// Cave-Diver, Nicanzil Current Conductor, Wildgrowth Walker, Lurking
// Chupacabra, Shadowed Caravel; 5 files / 6 raw lines at the corpus pin).
// The causing event is the completed events.Explore record: Obj is the
// EXPLORER (what ValidCard$ matches, with the explorer's controller as the
// event player — the same eventCardAndPlayerMatch read Sacrificed applies)
// and IDs[0] is the card the process revealed, which ValidExplored$ narrows
// ("explores a land card" / "explores a nonland card" — Nicanzil's pair).
// The revealed card is matched in whatever zone the explore left it in (hand
// or graveyard, or back on top): the plain type predicates both carriers use
// are zone-independent, and the reveal Note that precedes the record already
// made the card public, so no LKI capture is needed. A record with no
// revealed card is unreachable (an empty library records nothing).
func (e *Engine) exploresMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.Explore || len(ev.IDs) == 0 {
		return false
	}
	ctrl := e.controllerOf(source)
	if v := t.Params["ValidCard"]; v != "" &&
		!effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
		return false
	}
	if v := t.Params["ValidPlayer"]; v != "" &&
		!effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
		return false
	}
	if v := t.Params["ValidExplored"]; v != "" &&
		!effects.MatchesSpecCtx(e.G, v, ev.IDs[0], e.specCtx(source, ctrl)) {
		return false
	}
	return true
}

// counterAddedMatches implements the "when a counter is put on" trigger
// family (Forge Mode$ CounterAdded; Shang-Chi and the Ten Rings' "When the
// tenth +1/+1 counter is put on NICKNAME"). The gate is the CounterChange
// event that put counters (Amount > 0: a removal event never adds one).
// CounterType$ names the kind. CounterAmount$ <op><n> is the crossing gate
// the card text means: the trigger fires when the put takes the event's
// counter kind's total on the object from below n to at least n -- the tenth
// counter is put whether one event put 10 or a 5-then-5 pair crossed, and a
// batch that overshoots (9+2) also crossed it. A put that does not cross
// (7+1, 11+1) fires nothing. The threshold ops (EQ/GT/GE) all read as that
// one crossing -- the corpus's CounterAdded CounterAmount$ values are EQ only
// (EQ3..EQ12, every one an oracle "when the <Nth> counter is put" gate), so
// EQ is the measured shape and GT/GE collapse onto it unmeasured. An absent
// CounterAmount$ is Forge's plain "whenever a counter is put" -- every put
// admits it.
func (e *Engine) counterAddedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.CounterChange || ev.Amount <= 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	if kind := t.Params["CounterType"]; kind != "" && !strings.EqualFold(kind, ev.Counter) {
		return false
	}
	if !e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller) {
		return false
	}
	if cmp := t.Params["CounterAmount"]; cmp != "" {
		op, n, ok := splitCompare(strings.TrimSpace(cmp))
		if !ok {
			return false
		}
		after := o.Counter(ev.Counter)
		before := after - ev.Amount
		if before < 0 {
			before = 0
		}
		// The crossing semantics for the threshold ops: below n before, at
		// least n after. applyCompare(after, op, n) would miss an overshooting
		// batch (9+2 on EQ10: after=11 is not == 10) and re-fire on every
		// later put that lands exactly on n -- the oracle text ("when the
		// tenth counter is put") fires once, on the crossing, so EQ/GT/GE
		// collapse onto before < n && after >= n. LT/LE/NE do not occur in
		// the corpus on this mode; they keep the plain post-event comparison.
		switch op {
		case "EQ", "GT", "GE":
			if !(before < int32(n) && after >= int32(n)) {
				return false
			}
		default:
			if !applyCompare(int(after), op, n) {
				return false
			}
		}
	}
	return true
}

// counterRemovedMatches is CounterAdded's mirror for Mode$ CounterRemoved
// ("whenever a counter is removed from ~", "when the last <kind> counter is
// removed from ~"): the event is a CounterChange with a NEGATIVE Amount
// (effects/counters.go's removal primitives, rules/turn.go:64's suspend TIME
// upkeep decrement), filtered by CounterType$ (case-insensitive, same as the
// Added arm), ValidCard$/ValidPlayer$ and TriggerZones$ (the shared zoneGate
// already ran). CounterChange carries no player field, so ValidPlayer$ is
// matched against the object's controller -- the convention counterAddedMatches
// uses. NewCounterAmount$ is Forge's "the LAST counter" gate: the post-event
// total for the counter kind must equal the named value -- events.Apply folds
// the removal before checkTriggers runs, so o.Counter(ev.Counter) is already
// the total after. A malformed value fails closed (the Added arm's
// strconv/splitCompare style). Fire-once semantics are the event granularity:
// one Amount: -N batch removal is ONE trigger, exactly as CounterAdded fires
// once per CounterChange; the Once batch modes are separate.
func (e *Engine) counterRemovedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.CounterChange || ev.Amount >= 0 {
		return false
	}
	o := e.G.Obj(ev.Obj)
	if o == nil {
		return false
	}
	if kind := t.Params["CounterType"]; kind != "" && !strings.EqualFold(kind, ev.Counter) {
		return false
	}
	if !e.eventCardAndPlayerMatch(t, source, ev.Obj, o.Controller) {
		return false
	}
	if want := t.Params["NewCounterAmount"]; want != "" {
		n, err := strconv.Atoi(strings.TrimSpace(want))
		if err != nil {
			return false
		}
		if o.Counter(ev.Counter) != int32(n) {
			return false
		}
	}
	return true
}

// attackerBlockedCandidates lists the attackers one become-blocked trigger
// fires for (Forge Mode$ AttackerBlocked; She-Hulk, Wallbreaker's "Whenever
// a Hero you control becomes blocked"). A DeclareBlockers event's Pairs
// name exactly the attacker-blocker assignments this defender's declaration
// just made -- an attacker already carrying blockers is never re-paired, so
// the declared pairs ARE the became-blocked transition, and a trigger fires
// once per DISTINCT matching attacker (two Heroes blocked by one
// declaration are two trigger instances, CR 603.2c). Deterministic order:
// the event's own pair order, deduplicated.
func (e *Engine) attackerBlockedCandidates(t cards.Trigger, source state.ObjID, ev events.Event) []state.ObjID {
	if ev.Kind != events.DeclareBlockers || len(ev.Pairs) == 0 {
		return nil
	}
	ctrl := e.controllerOf(source)
	seen := map[state.ObjID]bool{}
	var out []state.ObjID
	for _, pr := range ev.Pairs {
		a := pr[0]
		if seen[a] {
			continue
		}
		seen[a] = true
		if v := t.Params["ValidCard"]; v != "" && !effects.MatchesSpecCtx(e.G, v, a, e.specCtx(source, ctrl)) {
			continue
		}
		out = append(out, a)
	}
	return out
}

// attackerBlockedByPairCandidates lists the (attacker, blocker) pairs one
// Forge Mode$ AttackerBlockedByCreature trigger fires for (kw:Flanking's
// expansion, CR 702.25a: "whenever this creature becomes blocked by a
// creature without flanking"). Each declared pair whose ATTACKER is the
// trigger's own source, matches ValidCard$, and whose blocker matches
// ValidBlocker$ yields one instance; a blocker WITH flanking matches nothing,
// so it debuffs nobody. ValidCard$ Card.Self works because the trigger's
// source IS the flanking attacker. The sibling "blocks" half of Forge's mode
// names the BLOCKER as its source and never reaches here (see the loop).
func (e *Engine) attackerBlockedByPairCandidates(t cards.Trigger, source state.ObjID, ev events.Event) [][2]state.ObjID {
	if ev.Kind != events.DeclareBlockers || len(ev.Pairs) == 0 {
		return nil
	}
	ctrl := e.controllerOf(source)
	var out [][2]state.ObjID
	for _, pr := range ev.Pairs {
		// The trigger's SOURCE must be the pair's ATTACKER. This hook binds the
		// blocker as the remembered object, so it is only correct for the
		// "becomes blocked" half of Forge's mode (kw:Flanking is its only live
		// carrier). The sibling "blocks" half spells its source as the BLOCKER
		// (ValidCard$ Creature | ValidBlocker$ Card.Self) and names the attacker
		// in its body (Defined$ TriggeredAttackerLKICopy); queueing it here would
		// resolve that referent to the remembered BLOCKER -- the source itself --
		// and make the creature damage/lose life to itself. That half stays inert
		// (role-correct referents need a second remembered slot, a separate task).
		if pr[0] != source {
			continue
		}
		if v := t.Params["ValidCard"]; v != "" && !effects.MatchesSpecCtx(e.G, v, pr[0], e.specCtx(source, ctrl)) {
			continue
		}
		if v := t.Params["ValidBlocker"]; v != "" && !effects.MatchesSpecCtx(e.G, v, pr[1], e.specCtx(source, ctrl)) {
			continue
		}
		out = append(out, pr)
	}
	return out
}

// checkAttackerBlockedTriggers queues trigger instances off a DeclareBlockers
// event for the two become-blocked modes the ordinary face scan cannot express
// (it queues at most one entry per trigger per event, and both referents are
// per-attacker or per-pair): Mode$ AttackerBlocked fires once per DISTINCT
// matching blocked attacker (She-Hulk's counter count is each Hero's OWN
// blocker count), and Forge Mode$ AttackerBlockedByCreature -- kw:Flanking's
// expansion (CR 702.25a) is its only live carrier -- fires once per matching
// (attacker, blocker) PAIR, the blocker remembered as the
// TriggeredBlockerLKICopy referent. The same-scan-hook precedent is
// checkChapterTriggers (rules/saga.go). The gates mirror the ordinary scan's
// per-trigger sequence (zone, phase, fire-count bound, ActivationLimit$);
// Secondary$ and the Once damage-batch gates do not exist on these modes.
// Each per-instance ctx carries the triggering objects as Remembered and as
// TriggerCard, so Count$Valid Creature.blockingTriggeredAttacker counts that
// Hero's blockers and Defined$ TriggeredBlockerLKICopy names the blocker.
func (e *Engine) checkAttackerBlockedTriggers(ev events.Event) {
	if ev.Kind != events.DeclareBlockers {
		return
	}
	pt := func(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil {
			return
		}
		f := o.Face()
		if f == nil {
			return
		}
		if !o.Unlocked && !e.faceMayTrigger(f, ev.Kind) {
			return
		}
		for ti, t := range f.Triggers {
			if t.Mode != "AttackerBlocked" && t.Mode != "AttackerBlockedByCreature" {
				continue
			}
			if !e.zoneGate(t, id, ev) || !e.phaseGate(t) {
				continue
			}
			key := triggerKey{Source: id, Idx: ti}
			if e.triggerFireCount == nil {
				e.triggerFireCount = map[triggerKey]int32{}
			}
			if e.triggerFireCount[key] >= maxTriggerFires {
				continue // cascade bound: see maxTriggerFires.
			}
			if actionTriggerModes[t.Mode] && !e.triggerActivationLimitAllows(t, key) {
				continue
			}
			if t.Mode == "AttackerBlockedByCreature" {
				// CR 702.25a: one instance per (attacker, blocker) pair; the
				// trigger's controller is the ATTACKER's controller, which the
				// Source/Controller pair already are (the source is the
				// flanking attacker itself).
				for _, pr := range e.attackerBlockedByPairCandidates(t, id, ev) {
					if t.Effect == nil {
						break
					}
					bid := pr[1]
					e.triggerFireCount[key]++
					e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
						Source:     id,
						Controller: o.Controller,
						Idx:        ti,
						SA:         t.Effect,
						Ctx: effects.Ctx{
							Source:     id,
							Controller: o.Controller,
							Remembered: []state.Target{{Obj: bid}},
							Captured:   []state.Target{{Obj: bid}},
							TriggerContext: effects.TriggerContext{
								TriggerCard:   bid,
								TriggerSource: pr[0],
							},
						},
					})
				}
				continue
			}
			for _, aid := range e.attackerBlockedCandidates(t, id, ev) {
				if t.Effect == nil {
					break
				}
				defender := pt(0)
				if ao := e.G.Obj(aid); ao != nil {
					defender = pt(ao.Attacking)
				}
				e.triggerFireCount[key]++
				e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
					Source:     id,
					Controller: o.Controller,
					Idx:        ti,
					SA:         t.Effect,
					Ctx: effects.Ctx{
						Source:     id,
						Controller: o.Controller,
						Remembered: []state.Target{{Obj: aid}},
						Captured:   []state.Target{{Obj: aid}},
						TriggerContext: effects.TriggerContext{
							TriggerCard:     aid,
							TriggerSource:   aid,
							AttackingPlayer: pt(e.controllerOf(aid)),
							DefendingPlayer: defender,
						},
					},
				})
			}
		}
	})
}

// exertedMatches is the trig:Exerted half of CR 702.100 (task exert1 built the
// election and the static's own Trigger$ rider; this is the separate "whenever
// you exert a creature" listener a different script line carries). The event is
// events.Exert: Obj is the permanent the controller exerted and Amount >= 0 is
// the exert itself, while Amount == -1 is the untap-step consume marker
// rules/turn.go's scan emits -- bookkeeping, never an exert, so it must not
// fire. The exerted permanent is still on the battlefield at match time, so
// ValidCard$ reads the live object against the trigger source's controller
// ("you" = the listener's controller; all five corpus carriers write
// Creature.YouCtrl). ValidPlayer$/ValidSource$ are not read: measured, none of
// the five corpus lines carries either.
func (e *Engine) exertedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Exert || ev.Amount < 0 {
		return false
	}
	ctrl := e.controllerOf(source)
	if v := t.Params["ValidCard"]; v != "" &&
		!effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
		return false
	}
	return true
}

// sacrificedMatches and discardedMatches identify the two actions from the
// existing, replayed zone-change event. Discard producers use events.Discard
// or events.DiscardCost, so the action marker and its cost provenance survive
// a replacement changing the destination.
func (e *Engine) sacrificedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if !events.IsSacrifice(ev) {
		return false
	}
	ctrl := e.controllerOf(source)
	if v := t.Params["ValidCard"]; v != "" {
		// A sacrificed permanent is already in its destination zone when
		// triggers are checked. Its validity -- especially bare Permanent --
		// is a last-known-information question at the moment it was sacrificed.
		if lki == nil || !effects.MatchesObjectCtx(e.G, v, lki, e.specCtx(source, ctrl)) {
			return false
		}
	}
	// The sacrificing player is the permanent's controller as it was
	// sacrificed (Forge GameAction.sacrifice reads the LKI): a stolen
	// permanent its taker sacrifices is the taker's sacrifice, although the
	// move has already returned it to its owner.
	sacrificer := e.controllerOf(ev.Obj)
	if lki != nil {
		sacrificer = lki.Controller
	}
	if v := t.Params["ValidPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, sacrificer, ctrl) {
		return false
	}
	return true
}

// tokenCreatedMatches identifies Forge's "whenever you create a token"
// trigger (Mode$ TokenCreated / TokenCreatedOnce) on the existing, replayed
// TokenCreate event -- one event per minted token (effects/token.go's effToken
// loop, effects/amass.go, and the token-replacement mint path), so a
// three-token spell fires the trigger three times and the Once mode's
// once-per-turn latch (triggerActivationLimitAllows, actionTriggerModes
// membership above) is what collapses a batch to one queueing. The would-be
// token does not exist as a game object at match time: ValidToken$ is taken
// against the same shallow tokenSnapshot the CreateToken replacement class
// matches against -- an unknown token key fails closed -- with the You-side
// predicates reading against the trigger source's controller while the
// snapshot's controller is ev.Player, the token's creator ("you create").
func (e *Engine) tokenCreatedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TokenCreate {
		return false
	}
	ctrl := e.controllerOf(source)
	if v := t.Params["ValidPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
		return false
	}
	if v, ok := t.Params["ValidToken"]; ok && v != "" {
		tok := e.tokenSnapshot(ev)
		if tok == nil || !effects.MatchesObjectCtx(e.G, v, tok, e.specCtx(source, ctrl)) {
			return false
		}
	}
	return true
}

// leftBattlefield reports a zone change whose object left the battlefield.
func leftBattlefield(ev events.Event) bool {
	return ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To != state.ZBattlefield
}

func (e *Engine) discardedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if !events.IsDiscard(ev) ||
		!e.eventCardAndPlayerMatch(t, source, ev.Obj, e.controllerOf(ev.Obj)) {
		return false
	}
	if spec := t.Params["ValidCause"]; spec != "" && !e.discardCauseAdmits(spec, source, ev) {
		return false
	}
	return true
}

// discardCauseAdmits evaluates a ValidCause$ stack spec against the spell or
// ability that caused discard ev, from source's controller's perspective. It
// serves both the Discarded trigger and a Discard$ True replacement.
func (e *Engine) discardCauseAdmits(spec string, source state.ObjID, ev events.Event) bool {
	// A discard paid as a cost has no causing spell or ability. In
	// particular, do not misattribute it to an unrelated object that was
	// already on the stack when a player activated in response.
	if events.IsDiscardCost(ev) {
		return false
	}
	cause := e.actionCause()
	if cause == 0 {
		return false
	}
	o := e.G.Obj(cause)
	return o != nil && state.StackKindAdmits(state.StackKindTokens(spec), state.StackKindOf(e.G, o), o,
		o.Controller, e.controllerOf(source))
}

// actionCause is the stack object whose resolving effect caused a synchronous
// action event. Costs are paid before an activated ability exists on the stack,
// so they deliberately have no cause and cannot satisfy ValidCause$. This is
// replay-safe: action triggers are checked synchronously inside emit, while
// the resolving object is still at the top of the replayed stack.
func (e *Engine) actionCause() state.ObjID {
	if len(e.G.Stack) == 0 {
		return 0
	}
	return e.G.Stack[len(e.G.Stack)-1]
}

// tapsMatches handles both becomes-tapped and tapped-for-mana triggers. A
// mana activation marks its cost Tap in the engine's synchronous context;
// ordinary Tap events deliberately do not, so attacking and a spell that taps
// a permanent never masquerade as producing mana.
//
// ValidPlayer$ (Taps) and Activator$ (TapsForMana) name the player who tapped
// the permanent -- Forge Card.tap's tapper -- not its controller: "whenever
// you tap an untapped creature an opponent controls" (Icewrought Sentry,
// Solitary Sanctuary, Hylda, Sharae) is about an opponent's creature that YOU
// tapped. emitTap supplies the tapper for every producer.
func (e *Engine) tapsMatches(t cards.Trigger, source state.ObjID, ev events.Event, forMana bool) bool {
	// A permanent entering tapped did not become tapped (CR 603.2e): neither
	// a library search's Tapped$ True entry nor an ETB$ True replacement body
	// can fire a Taps trigger.
	if ev.Kind != events.Tap || ev.Obj == 0 || e.tapIsEntryState(ev) ||
		(forMana && e.tappingForMana != ev.Obj) {
		return false
	}
	actor := e.tapActor(ev)
	if v := t.Params["Activator"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, actor, e.controllerOf(source)) {
		return false
	}
	if v := t.Params["Attacker"]; v != "" {
		want, err := strconv.ParseBool(v)
		if err != nil || e.G.Obj(ev.Obj) == nil || e.G.Obj(ev.Obj).IsAttacking != want {
			return false
		}
	}
	// FirstTime$ True: the permanent had not already become tapped this turn
	// (Forge's tappedThisTurn == 0). emit records the tap only after this
	// event's triggers are matched.
	if strings.EqualFold(t.Params["FirstTime"], "True") && e.becameTappedThisTurn(ev.Obj) {
		return false
	}
	if forMana && !tapsForManaProduced(t.Params["Produced"], e.tappingManaProduced) {
		return false
	}
	return e.eventCardAndPlayerMatch(t, source, ev.Obj, actor)
}

// tapIsEntryState reports whether a Tap event only gives a permanent the
// tapped state it enters with: a library search's Tapped$ True entry (marked
// in the replayed payload) or an ETB$ True replacement body (marked in
// emitTap's context, the payload being the ordinary Tap).
func (e *Engine) tapIsEntryState(ev events.Event) bool {
	return ev.Text == "entered tapped" || (e.tapObj == ev.Obj && e.tapEntering)
}

// tapActor is the player who tapped ev's permanent. A Tap emitted without
// emitTap provenance falls back to the permanent's controller.
func (e *Engine) tapActor(ev events.Event) state.PlayerID {
	if e.tapObj == ev.Obj && ev.Obj != 0 {
		return e.tapPlayer
	}
	return e.controllerOf(ev.Obj)
}

// becameTappedThisTurn reports whether obj already became tapped this turn.
func (e *Engine) becameTappedThisTurn(obj state.ObjID) bool {
	turn, ok := e.tappedTurn[obj]
	return ok && turn == e.G.Turn
}

// tapsForManaProduced matches a TapsForMana Produced$ restriction against
// the activating ability's Produced$ declaration the way Forge does against
// the produced mana: the trigger fires when that mana CONTAINS the named
// type, so Forsaken Monument's Produced$ C fires for "C C" and "C U" as well
// as "C". A declaration whose output is a player's choice (Any, Combo ...,
// Chosen) never produces colourless mana, and the chosen colour is not known
// at the Tap boundary, so a colour-choice declaration matches nothing; a
// ChosenColor restriction (the trigger's own chosen colour) is likewise
// unsupported and fails closed.
func tapsForManaProduced(want, produced string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return true
	}
	if len(want) != 1 || !strings.Contains(effects.ManaSymbols, want) {
		return false
	}
	for _, field := range strings.Fields(strings.NewReplacer("{", " ", "}", " ").Replace(produced)) {
		for _, r := range field {
			if !strings.ContainsRune(effects.ManaSymbols, r) {
				return false // a choice word: Any, Combo, Chosen, ...
			}
		}
		if strings.Contains(field, want) {
			return true
		}
	}
	return false
}

// commitCrimeMatches implements CR 700.13: targeting an opponent, a
// permanent they control, or a card in their graveyard commits one crime.
// A multi-target spell produces one TargetsChosen event per target. After
// Apply, append events can inspect the prior targets already on the stack;
// only the first criminal target may fire this trigger.
func (e *Engine) commitCrimeMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	actor := e.controllerOf(ev.Obj)
	if !e.targetEventCommitsCrime(ev, actor) {
		return false
	}
	if o := e.G.Obj(ev.Obj); o != nil && (ev.Amount == 2 || ev.Amount == 3) {
		for _, target := range o.Targets[:len(o.Targets)-1] {
			if e.targetCommitsCrime(target, actor) {
				return false
			}
		}
	}
	if v := t.Params["ValidPlayer"]; v != "" {
		return effects.MatchesPlayerSpec(e.G, v, actor, e.controllerOf(source))
	}
	return true
}

func (e *Engine) targetEventCommitsCrime(ev events.Event, actor state.PlayerID) bool {
	if ev.Amount == 1 || ev.Amount == 3 {
		return e.targetCommitsCrime(state.Target{Player: ev.Player, IsPlayer: true}, actor)
	}
	return len(ev.IDs) == 1 && e.targetCommitsCrime(state.Target{Obj: ev.IDs[0]}, actor)
}

// targetCommitsCrime is CR 700.13's list, and only that list: an opponent; a
// permanent or a spell or ability on the stack an opponent controls; or a card
// in an opponent's graveyard, which is judged by its owner (CR 108.4a: a card
// that is not a permanent or spell has no controller). A card in exile, a
// hand or a library is none of these, whoever owns it.
func (e *Engine) targetCommitsCrime(target state.Target, actor state.PlayerID) bool {
	if target.IsPlayer {
		return target.Player != actor && int(target.Player) < len(e.G.Players)
	}
	o := e.G.Obj(target.Obj)
	if o == nil {
		return false
	}
	switch o.Zone {
	case state.ZBattlefield, state.ZStack:
		return o.Controller != actor
	case state.ZGraveyard:
		return o.Owner != actor
	}
	return false
}

// eventCardAndPlayerMatch applies the shared ValidCard$/ValidPlayer$ clauses
// on action triggers. The player is the player who performed the action.
func (e *Engine) eventCardAndPlayerMatch(t cards.Trigger, source, card state.ObjID, player state.PlayerID) bool {
	ctrl := e.controllerOf(source)
	if v := t.Params["ValidCard"]; v != "" && !effects.MatchesSpecCtx(e.G, v, card, e.specCtx(source, ctrl)) {
		return false
	}
	if v := t.Params["ValidPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, player, ctrl) {
		return false
	}
	return true
}

// damageSource identifies who dealt a just-emitted Damage event, for
// ValidSource$ matching. events.Event carries no explicit source field for
// Damage -- every Damage event this build emits (effects/damage.go's
// DealDamage/DamageAll) comes from a primitive running inside Resolve,
// called only from resolveTop while the resolving spell or ability is still
// the top of the stack (resolveTop pops it only after Resolve returns), so
// the current stack top is that source for every code path this build has
// today. Two overrides win over the stack top, both rebuilt by replay
// because replay re-executes the same setter: the published damage-source
// override (rules.Engine.SetDamageSource -- DamageSource$ and the unwrapped
// ability source, so a ValidSource$ trigger matches the PERMANENT that dealt
// it, never the ability wrapper the stack top names) and the dealing
// creature during combat's assignment loop (e.damaging). Any Damage emission
// outside ability resolution would need Event to carry an explicit source
// instead of relying on this.
func (e *Engine) damageSource() state.ObjID {
	if e.dmgSrcOverride != 0 {
		return e.dmgSrcOverride
	}
	if len(e.G.Stack) == 0 {
		return 0
	}
	return e.G.Stack[len(e.G.Stack)-1]
}

// drawnMatches implements Mode$ Drawn. A Draw event moves exactly one card
// from a library to its controller's hand, so ValidCard$ is tested against the
// drawn object and TriggeredPlayer is that event's Player. FirstCardInDrawStep$
// is derived from the ordered log after the event has landed: only the first
// Draw between entry to the draw step and its next StepChange qualifies.
func (e *Engine) drawnMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Draw {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidCard"]; ok && !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
		return false
	}
	if v := t.Params["ValidPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
		return false
	}
	if v := t.Params["Number"]; v != "" {
		want, err := strconv.Atoi(v)
		if err != nil || e.drawNumberThisTurn(ev.Player) != want {
			return false
		}
	}
	// PlayerTurn$ is the trigger controller's turn, not the drawing player's:
	// Keranos's "on each of your turns" must reject an opponent's first draw.
	if strings.EqualFold(t.Params["PlayerTurn"], "True") && e.G.Active != ctrl {
		return false
	}
	if v, ok := t.Params["FirstCardInDrawStep"]; ok {
		first := e.firstCardInDrawStep(ev.Player)
		if (strings.EqualFold(v, "True") && !first) || (strings.EqualFold(v, "False") && first) {
			return false
		}
	}
	return true
}

// drawNumberThisTurn counts p's draws in the current turn, including the Draw
// event currently being matched. The log is the replay-stable source of this
// per-turn fact; each player has its own ordinal because "their second card"
// must not count another seat's draw.
func (e *Engine) drawNumberThisTurn(p state.PlayerID) int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

// firstCardInDrawStep reports whether the most recently emitted Draw for p is
// the first draw since this turn entered its draw step. The log, rather than a
// mutable counter, is the source of this ephemeral fact so cloning and replay
// rebuild it without an event-schema change.
func (e *Engine) firstCardInDrawStep(p state.PlayerID) bool {
	if e.G.Step != state.StepDraw {
		return false
	}
	draws := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.Draw && ev.Player == p {
			draws++
		}
		if ev.Kind == events.StepChange {
			return ev.Step == state.StepDraw && draws == 1
		}
	}
	return false
}

// pendingDrawIsFirstInDrawStep reports whether a Draw about to be logged for
// p is the first p draws since this turn entered its draw step. It is the
// pre-emit twin of firstCardInDrawStep: replacement matching runs from
// Engine.emit BEFORE the proposed Draw is appended to e.L.Events, so the
// pending event itself is the "next" draw (draw count 0) rather than a
// logged one (draw count 1). It requires p to be the ACTIVE player as well,
// because the exempt draw CR 504.1 grants is that player's own turn-based
// draw: a non-active player drawing during someone else's draw step is not
// the first one they draw in each of their own draw steps, so Notion Thief
// and Hullbreacher must still replace it. firstCardInDrawStep deliberately
// omits that active-player test (a trigger reads whoever drew); the two
// cannot share a body, so they are kept adjacent with identical log-scan
// shapes to stop the pair drifting.
func (e *Engine) pendingDrawIsFirstInDrawStep(p state.PlayerID) bool {
	if e.G.Step != state.StepDraw || p != e.G.Active {
		return false
	}
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.Draw && ev.Player == p {
			return false
		}
		if ev.Kind == events.StepChange {
			return ev.Step == state.StepDraw
		}
	}
	return false
}

// lifeLoss names the player and positive magnitude of an event that lowers a
// player's life total. Damage to a player and a negative LifeChange are both
// loss of life; damage to an object is not.
func lifeLoss(ev events.Event) (state.PlayerID, int32, bool) {
	switch ev.Kind {
	case events.Damage:
		if ev.Obj == 0 && ev.Amount > 0 {
			return ev.Player, ev.Amount, true
		}
	case events.LifeChange:
		if ev.Amount < 0 {
			return ev.Player, -ev.Amount, true
		}
	}
	return 0, 0, false
}

// lifeLostMatches implements Mode$ LifeLost and LifeLostAll. LifeLost sees
// each losing player. LifeLostAll is deferred by Begin/EndLifeLossBatch and
// matches exactly once after adding every serialized loss for each player in
// the simultaneous group. A combat assignment may serialize two one-damage
// hits to one player, but that player lost two life in the one event.
func (e *Engine) lifeLostMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	ctrl := e.controllerOf(source)
	if t.Mode == "LifeLostAll" && e.finishingLifeLossBatch {
		amounts := make([]int32, len(e.G.Players))
		for _, be := range e.lifeLossBatch {
			p, amount, ok := lifeLoss(be)
			if ok && int(p) < len(amounts) {
				amounts[p] += amount
			}
		}
		matched := false
		for p, amount := range amounts {
			if amount == 0 {
				continue
			}
			player := state.PlayerID(p)
			if v := t.Params["ValidPlayer"]; v != "" && !effects.MatchesPlayerSpec(e.G, v, player, ctrl) {
				continue
			}
			if v := t.Params["ValidAmountEach"]; v != "" && !compareLife(amount, v) {
				return false
			}
			matched = true
		}
		return matched
	}
	p, amount, ok := lifeLoss(ev)
	if !ok {
		return false
	}
	if v, ok := t.Params["ValidPlayer"]; ok && !effects.MatchesPlayerSpec(e.G, v, p, ctrl) {
		return false
	}
	if v, ok := t.Params["ValidAmountEach"]; ok && !compareLife(amount, v) {
		return false
	}
	// On LifeLost, LifeAmount$ describes the amount just lost, not the
	// LifeTotal$ intervening-if grammar used by other trigger modes.
	if v := t.Params["LifeAmount"]; v != "" && !compareLife(amount, v) {
		return false
	}
	if strings.EqualFold(t.Params["PlayerTurn"], "True") && e.G.Active != ctrl {
		return false
	}
	if v := t.Params["ValidCause"]; v != "" && !e.lifeLossCauseMatches(v, ctrl) {
		return false
	}
	if strings.EqualFold(t.Params["FirstTime"], "True") && !e.firstLifeLossThisTurn(p) {
		return false
	}
	return true
}

// lifeLossCauseMatches recognizes the spell/ability cause grammar carried by
// LifeLost triggers. Events intentionally do not encode an extra source field,
// so a synchronous trigger read uses the Engine's in-flight resolving source;
// it is set around every stack resolution and cleared afterward, and thus
// replay derives the same answer. Combat damage is a creature cause, not a
// SpellAbility cause.
func (e *Engine) lifeLossCauseMatches(spec string, you state.PlayerID) bool {
	if e.combatDamaging || e.damaging == 0 {
		return false
	}
	cause := e.G.Obj(e.damaging)
	if cause == nil {
		return false
	}
	for _, alt := range strings.Split(spec, ",") {
		base, qualifier, qualified := strings.Cut(strings.TrimSpace(alt), ".")
		if base != "SpellAbility" {
			continue
		}
		if !qualified || qualifier == "" {
			return true
		}
		switch qualifier {
		case "YouCtrl":
			if cause.Controller == you {
				return true
			}
		case "OppCtrl":
			if cause.Controller != you {
				return true
			}
		}
	}
	return false
}

// firstLifeLossThisTurn is true only for the newest life-loss event of p in
// the current turn. TurnChange is the logged reset boundary for every other
// per-turn fact, so scanning back to it is replay-stable and cannot leak a
// mutable counter across Clone.
func (e *Engine) firstLifeLossThisTurn(p state.PlayerID) bool {
	seenCurrent := false
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			return seenCurrent
		}
		q, _, ok := lifeLoss(ev)
		if !ok || q != p {
			continue
		}
		if seenCurrent {
			return false
		}
		seenCurrent = true
	}
	return seenCurrent
}

// damageMatches implements Mode$ DamageDone, DamageDealtOnce and
// DamageDoneOnce (the once-per-damage-batch gate itself lives in
// checkTriggers, alongside the cascade bound; this is purely the per-event
// parameter match, shared by all three modes).
func (e *Engine) damageMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Damage {
		return false
	}
	// CombatDamage$ splits the mode between combat and noncombat damage
	// (CR 702.1x: combat damage is what the combat damage step's attackers
	// and blockers assign -- a DealDamage cast during that step is still not
	// combat damage). events.Event deliberately carries no such flag -- its
	// binary encoding is hash-chained and replayed -- so the distinction is
	// e.combatDamaging (engine.go), set only around dealCombatDamage's
	// assignment loop (combat.go) and read here synchronously inside emit's
	// checkTriggers; replay rebuilds it by re-executing the same setter.
	// CombatDamage$ False is the complement (16 corpus trigger lines): only
	// noncombat damage, so an in-flight combat assignment fails it. Before
	// the flag existed True returned false unconditionally (978 dead corpus
	// trigger lines, Umezawa's Jitte among them) and False fell through and
	// matched everything.
	switch cd := t.Params["CombatDamage"]; {
	case strings.EqualFold(cd, "True") && !e.combatDamaging:
		return false
	case strings.EqualFold(cd, "False") && e.combatDamaging:
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidSource"]; ok {
		// The damage's source, in priority order:
		//  1. an explicit published override (rules.Engine.SetDamageSource --
		//     DamageSource$ names the PERMANENT that dealt it, never the
		//     ability wrapper resolving it) -- authoritative whenever an
		//     emitter set one, combat included.
		//  2. during combat's assignment loop, e.damaging (the actual
		//     attacker/blocker dealing this hit). The stack is USUALLY empty
		//     during combat, but not always -- the between-passes priority
		//     round (CR 510.3/4) can leave a first-strike trigger on the
		//     stack while the regular pass deals (measured:
		//     TestUmezawasJitteGainsChargeCountersPerDamageStep's bearer
		//     deals in both passes with the first pass's trigger unresolved
		//     on the stack) -- so combat damage must prefer e.damaging over
		//     the stack top, or every ValidSource$ CombatDamage$ trigger
		//     (Umezawa's Jitte's ValidSource$ Creature.EquippedBy among them)
		//     goes dead for the second pass.
		//  3. otherwise, the resolving spell or ability while it is the
		//     stack top (damageSource).
		src := e.dmgSrcOverride
		if src == 0 {
			if e.combatDamaging {
				src = e.damaging
			} else {
				src = e.damageSource()
			}
		}
		if src == 0 || !effects.MatchesSpecCtx(e.G, v, src, e.specCtx(source, ctrl)) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		if ev.Obj != 0 {
			if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
				return false
			}
		} else if !effects.MatchesPlayerSpecFrom(e.G, v, ev.Player, ctrl, source) {
			return false
		}
	}
	return true
}

// damagePreventedMatches implements Mode$ DamagePreventedOnce (task dponce1):
// the trigger fires on a STORED prevention Note -- the re-entrant Note the
// full-prevention replacement arm (rules/replacement.go
// applyNonMoveReplacements) and the ReplaceDamage/protection siblings emit
// when damage is prevented. The Note carries the prevented damage in Amount
// (0 for Fog's whole-pass statement, which is deliberately excluded -- a
// whole-turn statement is not "damage that would be dealt to you is
// prevented") and names the damaged side in Obj/Player exactly like the
// DamageDone trigger's event does, so ValidTarget$ reads the same grammar:
// the damaged object when the hit was object-directed, the damaged player
// otherwise. There is no Once latch: each stored prevention Note is one
// occurrence, so two prevented hits in one turn fire twice, each with its
// own amount.
func (e *Engine) damagePreventedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Note || ev.Amount <= 0 {
		return false
	}
	if !strings.Contains(strings.ToLower(ev.Text), "prevent") {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidTarget"]; ok {
		if ev.Obj != 0 {
			if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
				return false
			}
		} else if !effects.MatchesPlayerSpecFrom(e.G, v, ev.Player, ctrl, source) {
			return false
		}
	}
	return true
}

func (e *Engine) flippedCoinMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	flipper, win, ok := effects.FlipNoteResult(ev)
	if !ok {
		return false
	}
	// ValidResult$ gates the side: Win = heads (Amount 1), Lose = tails
	// (Amount 0). Every corpus FlippedCoin line carries one; an absent
	// ValidResult$ (no such line measured) would fire on both sides.
	if res := strings.TrimSpace(t.Params["ValidResult"]); res != "" {
		if strings.EqualFold(res, "Win") && !win {
			return false
		}
		if strings.EqualFold(res, "Lose") && win {
			return false
		}
	}
	// ValidPlayer$ names the FLIPPER ("whenever YOU win a coin flip"): the
	// Note's Player, through the shared player-spec grammar with the trigger's
	// own controller as You. The two "whenever a player wins" lines carry no
	// ValidPlayer$ and fire on any flipper.
	if v, ok := t.Params["ValidPlayer"]; ok {
		if !effects.MatchesPlayerSpecFrom(e.G, v, flipper, e.controllerOf(source), source) {
			return false
		}
	}
	return true
}

// attachedMatches implements Mode$ Attached: the trigger fires when an Aura,
// Equipment or other attachment becomes attached to a permanent (CR
// 701.3a's "becomes attached" -- the event the engine's one shared attach
// emit site, effects/attach.go's effAttach, publishes for the cast, equip
// and ETB-attached shapes alike). events.Attach with len(ev.IDs) > 0 carries
// the attachment in ev.Obj and the bearer in ev.IDs[0]; the no-IDs emits are
// the detach state-based actions (rules/attach.go), which are NOT "becomes
// attached" and never match, and an emit with no attachment object (Obj == 0)
// has nothing to bind ValidSource$ against, so it never matches either.
// Forge's ValidSource$ names the ATTACHING
// object (Siona's Aura.YouCtrl, Enormous Energy Blade's Card.Self) and
// ValidTarget$ names the BEARER (Brood Keeper's Card.Self reads Self as the
// trigger's source through the same specCtx becomesTargetMatches uses).
// A trigger with NO ValidTarget$ never fires: Eriette's line names only
// TargetRelativeToSource$, a parameter this build does not read anywhere,
// so firing without the bearer restriction would over-fire on every Aura
// attach. The Static$ True guard keeps Forge's "static effect expressed as
// a trigger" lines (Metamorphic Alteration, Paleontologist's Pick-Axe --
// Execute$ DBClone continuous shapes with no static-trigger machinery here)
// from firing a clone on every attach.
func (e *Engine) attachedMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.Attach || len(ev.IDs) == 0 || ev.Obj == 0 {
		return false
	}
	if t.Params["Static"] == "True" {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidSource"]; ok {
		if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		return effects.MatchesSpecCtx(e.G, v, ev.IDs[0], e.specCtx(source, ctrl))
	}
	return false
}

// ringTemptsMatches implements Mode$ RingTemptsYou (CR 701.54d): the trigger
// fires when the Ring tempts its controller, "when the actions complete,
// even if some were impossible" — an event with Obj 0 (no creature was
// designated) still counts as a temptation. The tempted player is ev.Player;
// ValidPlayer$ gates on the tempted player against the source's controller
// (the corpus's only spelling, `ValidPlayer$ You`). ValidCard$ gates on the
// chosen Ring-bearer with the source's controller as "you" and the source as
// Other — the corpus's `Creature.YouCtrl+Other` shape ("a creature other than
// CARDNAME") and the plain `Creature.YouCtrl` shape both resolve through it;
// with no designated bearer a ValidCard$ trigger never fires.
func (e *Engine) ringTemptsMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.RingTemptsYou {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	if v := t.Params["ValidCard"]; v != "" {
		if ev.Obj == 0 {
			return false // no creature became the Ring-bearer
		}
		if !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, ctrl)) {
			return false
		}
	}
	return true
}

// becomesTargetMatches implements Mode$ BecomesTarget: the trigger fires
// when one of the chosen targets recorded by a TargetsChosen event
// (rules.handleTarget -- "the target decision being answered") matches its
// ValidTarget$ -- or, with no ValidTarget$, when its own source is among the
// targets. Forge's ValidTarget$ names the TARGETED object: the self-shapes
// (ValidTarget$ Card.Self, the ward family, Reality Smasher) match their own
// source that way, and the "a Dragon you control becomes the target" shapes
// (Thunderbreak Regent) match a target their source merely watches -- the
// 50 non-self corpus lines of the 132-line mode.
func (e *Engine) becomesTargetMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.TargetsChosen {
		return false
	}
	if v, ok := t.Params["ValidSource"]; ok {
		// ValidSource$ names the spell or ability doing the targeting: the
		// TargetsChosen event's Obj, the stack object whose target decision
		// this event answers (commitCrimeMatches reads the same field as the
		// targeting actor). Reality Smasher's "spell an opponent controls"
		// (Spell.OppCtrl) and Thunderbreak Regent's "spell or ability"
		// (SpellAbility.OppCtrl) both resolve against that stack object; an
		// event carrying no targeting object can never match.
		if ev.Obj == 0 || !effects.MatchesSpecCtx(e.G, v, ev.Obj, e.specCtx(source, e.controllerOf(source))) {
			return false
		}
	}
	if v, ok := t.Params["ValidTarget"]; ok {
		for _, id := range ev.IDs {
			if effects.MatchesSpecCtx(e.G, v, id, e.specCtx(source, e.controllerOf(source))) {
				// CR 702.21a compares the Ward permanent's controller with the
				// controller of the targeting spell or ability ON THE STACK
				// (the same ev.Obj ValidSource$ reads above). For an ability,
				// protectionSource would unwrap ev.Obj to its source permanent,
				// whose controller may have changed since activation. Every
				// ward trigger targets only itself (the synthesized shape,
				// trigger_match.go's ward expansion), so this gate runs on the
				// self match.
				if t.Params["Ward"] == "True" &&
					(ev.Obj == 0 || e.controllerOf(ev.Obj) == e.controllerOf(source)) {
					return false
				}
				return true
			}
		}
		return false
	}
	targeted := false
	for _, id := range ev.IDs {
		if id == source {
			targeted = true
			break
		}
	}
	if targeted && t.Params["Ward"] == "True" &&
		(ev.Obj == 0 || e.controllerOf(ev.Obj) == e.controllerOf(source)) {
		// The ValidTarget$ branch's ward gate, applied to the bare
		// self-targeted fallback: a ward trigger never fires for its own
		// controller's targeting (CR 702.21a compares the ward permanent's
		// controller with the targeting spell or ability's, the same ev.Obj
		// ValidSource$ reads above). Unreachable in the current corpus --
		// every ward trigger is keyword-synthesized with ValidTarget$
		// Card.Self (cards/keywords.go, 0 raw Ward$ True lines) -- kept so
		// a future ward trigger without ValidTarget$ cannot fire for its
		// own controller.
		return false
	}
	return targeted
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

type parsedPhase struct {
	set   state.StepSet
	valid bool
}

// parsedPhaseSpec caches syntax only, never whether the current step matches.
// Diagnostic scans and live/look-back matchers use the same parse semantics;
// only the live scan emits Notes, tracked separately in phaseUnknownNoted.
func (e *Engine) parsedPhaseSpec(spec string) parsedPhase {
	if p, ok := e.phaseSpecs[spec]; ok {
		return p
	}
	set, unknown := state.ParsePhases(spec)
	p := parsedPhase{set: set, valid: len(unknown) == 0}
	if e.phaseSpecs == nil {
		e.phaseSpecs = make(map[string]parsedPhase)
	}
	e.phaseSpecs[spec] = p
	return p
}

// phaseGate applies Forge's Phase$ (validPhases) uniformly to every trigger
// mode. It is deliberately before the mode switch in triggerMatches: a
// ChangesZone or SpellCast trigger with Phase$ Main1 must not fire during an
// upkeep, and an unresolvable name fails closed. checkFaceTriggers reports
// that invalid name once as a Note; this bool-only matcher does not emit
// while it may be walking a scratch look-back observer. An absent Phase$
// remains ungated, matching Forge's null validPhases.
//
// PhaseCount$ narrows a Phase$ set to the Nth member of that set in turn
// order: `Phase$ Main | PhaseCount$ 2` is the SECOND main phase, so the gate
// fails at the first. A non-positive or non-numeric value fails closed (the
// conservative direction -- the trigger then fires at no step rather than
// every matching one).
func (e *Engine) phaseGate(t cards.Trigger) bool {
	spec := t.Params["Phase"]
	if strings.TrimSpace(spec) == "" {
		return true
	}
	p := e.parsedPhaseSpec(spec)
	if !p.valid || !p.set.Has(e.G.Step) {
		return false
	}
	count := strings.TrimSpace(t.Params["PhaseCount"])
	if count == "" {
		return true
	}
	n, err := strconv.Atoi(count)
	if err != nil || n < 1 {
		return false
	}
	return p.set.Ordinal(e.G.Step) == n
}

// phaseMatches implements Mode$ Phase after phaseGate has already checked
// its Phase$ parameter. The mode itself is only a StepChange event plus its
// optional ValidPlayer$ restriction.
func (e *Engine) phaseMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StepChange {
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
	// HasXManaCost$ True: the activation cost must contain {X}. The ability
	// at the recorded index (the same bounds check ValidSA$ uses); a stale
	// index fails closed through the nil ab below.
	var ab *cards.SA
	if ev.Amount >= 0 && int(ev.Amount) < len(obj.Face().Abilities) {
		ab = obj.Face().Abilities[int(ev.Amount)]
	}
	if !hasXManaCostGate(t.Params, "", ab) {
		return false
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
				if !isManaAbilityAPI(ab.API) {
					return true
				}
			case "ManaAbility":
				if isManaAbilityAPI(ab.API) {
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
//   - Metalcraft$ True (and the bare-Condition$ Metalcraft spelling): the
//     controller controls three or more artifacts -- the trigger-side named
//     condition the ability word uses (6 corpus files: Vedalken Humiliator's
//     attack pump, Blade-Tribe Berserkers' and Bleak Coven Vampires' ETBs,
//     Lumengrid Drake's bounce, Inventors' Fair's upkeep lifegain, Screeching
//     Silcaw's mill), read through the SAME metalcraftHolds census
//     costConditionHolds' Condition$ Metalcraft case and the Continuous
//     static gate read, so the three cannot drift apart.
//
// An absent clause is vacuously true. A clause whose shape this build cannot
// evaluate FAILS CLOSED -- a false condition means the trigger simply does not
// fire, never that an unreadable life/creature count is presumed large
// enough to let a win or counter trigger slip through.
func (e *Engine) triggerConditionHolds(t cards.Trigger, source state.ObjID) bool {
	return e.triggerConditionHoldsAs(t, source, e.controllerOf(source))
}

// triggerConditionHoldsAs is triggerConditionHolds with "you" supplied
// explicitly rather than derived from source's controller. An event-matched
// delayed trigger's "you" is the registration's effect owner (dt.Controller),
// which can differ from the source card's own controller -- see
// checkEventDelayedTriggers.
func (e *Engine) triggerConditionHoldsAs(t cards.Trigger, source state.ObjID, you state.PlayerID) bool {
	// LifeLost's LifeAmount$ is matched against the causing loss by
	// lifeLostMatches, rather than against a player's current life total.
	if v, ok := t.Params["LifeAmount"]; ok && t.Mode != "LifeLost" && t.Mode != "LifeLostAll" {
		if !e.lifeConditionHoldsAs(t, you, v) {
			return false
		}
	}
	if spec, ok := t.Params["IsPresent"]; ok {
		cmp, hasCmp := t.Params["PresentCompare"]
		// PresentDefined$ names the base set the IsPresent$ spec is counted
		// over (Mana Vault's "if this artifact is tapped": PresentDefined$
		// Self narrows the scan to the source itself, where the old whole-
		// battlefield walk counted every tapped permanent). An absent
		// PresentDefined keeps the historic whole-battlefield scan. A value
		// that is not the source fails closed with the rest of the clause.
		if pd := strings.TrimSpace(t.Params["PresentDefined"]); pd != "" && pd != "Self" {
			return false
		}
		if !hasCmp {
			// Forge's own reading of an IsPresent$ clause with no
			// PresentCompare$ is "at least one match" (Mana Vault's draw-step
			// damage): a present-condition with no comparison never meant
			// "vacuously true", which is what the old hard return made it.
			spec2 := strings.TrimSpace(t.Params["IsPresent2"])
			// PresentZone$ scopes the count to one named zone (Jocasta's
			// "if this card is in your graveyard"); the no-compare branch
			// honours it exactly like the compared branch below. A zone word
			// this build does not know, or a combination with the IsPresent2$
			// union whose zone each member would scan, fails closed.
			if pz := strings.TrimSpace(t.Params["PresentZone"]); pz != "" {
				if spec2 != "" {
					return false
				}
				return e.countPresentZone(t, spec, source, you)
			}
			if spec2 != "" {
				return e.presentUnionCount(spec, spec2, source, you) > 0
			}
			return e.countPresent(spec, source, you) > 0
		}
		if !e.presentConditionHoldsAs(t, source, you, spec, cmp) {
			return false
		}
		// IsPresent2$ names a SECOND present set whose objects count alongside
		// IsPresent$'s, as one union ("Name Sticker" Goblin counts creatures
		// named Name Sticker Goblin plus the entering one; the source itself
		// sits in both sets, so a plain sum would count it twice and break the
		// boundary the comparison guards). Deduplicating by object identity is
		// the only reading that reproduces the card's "9 or fewer creatures
		// named ..." at every count.
		if spec2 := strings.TrimSpace(t.Params["IsPresent2"]); spec2 != "" {
			op, n, ok := splitCompare(strings.TrimSpace(cmp))
			if !ok {
				return false
			}
			return applyCompare(e.presentUnionCount(spec, spec2, source, you), op, n)
		}
	}
	if v, ok := t.Params["Metalcraft"]; ok {
		// The trigger-side named condition (task trig-attacks-metalcraft):
		// a value this build cannot read as True is an unreadable clause
		// shape and fails closed like the other clauses above.
		if !strings.EqualFold(strings.TrimSpace(v), "True") || !e.metalcraftHolds(you) {
			return false
		}
	}
	if strings.EqualFold(strings.TrimSpace(t.Params["Condition"]), "Metalcraft") {
		// The bare-Condition$ spelling of the same gate. The trigger path
		// reads no OTHER bare Condition$ value (LifePaid, Evolve,
		// Sacrificed and friends are matched by their own per-kind helpers
		// or stay unread), and no corpus trigger carries this spelling
		// today -- the bare Condition$ Metalcraft carriers are S: statics
		// the Continuous gate already reads -- but the spelling is kept
		// beside Metalcraft$ so the two cannot drift apart.
		if !e.metalcraftHolds(you) {
			return false
		}
	}
	if v, ok := t.Params["Revolt"]; ok {
		// Revolt$ (the CR 702.38 ability word, "if a permanent you
		// controlled left the battlefield this turn"): the SAME
		// revoltThisTurn scan the replacement path's Revolt$ clause reads
		// (rules/replacement.go), the one Engine.RevoltHolds -- effects'
		// bare Condition$ Revolt gate and Count$Revolt branch head --
		// delegates to, so the three spellings cannot drift apart. Measured
		// over the corpus: 29 files carry Revolt$ True; 24 of its lines are
		// triggers -- 16 Mode$ Phase end-step shapes (Aid from the Cowl,
		// Hidden Stockpile, Krang) and 8 Mode$ ChangesZone ETBs (Airdrop
		// Aeronauts, Vengeful Rebel, Deadeye Harpooner); the rest are a
		// replacement line (Aether Revolt, already read on the replacement
		// path) and non-trigger text. A value this build cannot read as
		// True is an unreadable clause shape and fails closed like the
		// Metalcraft$ clause above.
		if !strings.EqualFold(strings.TrimSpace(v), "True") || !e.revoltThisTurn(you) {
			return false
		}
	}
	if strings.EqualFold(strings.TrimSpace(t.Params["Condition"]), "Revolt") {
		// The bare-Condition$ spelling of the same gate. No corpus trigger
		// carries it today (the one bare Condition$ Revolt carrier,
		// Decommission, is a DB$ GainLife sub the effects condition gate
		// reads), kept beside the bare Condition$ Metalcraft spelling so
		// the two cannot drift apart.
		if !e.revoltThisTurn(you) {
			return false
		}
	}
	if name, ok := t.Params["CheckSVar"]; ok {
		// CheckSVar$/SVarCompare$ (Kozilek, the Great Distortion's cast
		// trigger: "if you have fewer than seven cards in hand"): the
		// CR 603.4 intervening-if the shared SVar-compare evaluator reads,
		// evaluated with the source face's SVar table and the trigger's
		// "you" -- the same wrapper rules' static gate uses. A condition
		// this build cannot evaluate fails closed (the trigger does not
		// fire), the same convention triggerConditionHolds' other clauses
		// document above.
		src := e.G.Obj(source)
		if src == nil || src.Face() == nil {
			return false
		}
		ctx := &effects.Ctx{Source: source, Controller: you, SVars: src.Face().SVars}
		holds, evaluated := effects.CheckSVarHolds(e, ctx, name, strings.TrimSpace(t.Params["SVarCompare"]))
		if !evaluated || !holds {
			return false
		}
	}
	if spec, ok := t.Params["CheckDefinedPlayer"]; ok {
		holds, supported := e.checkDefinedPlayerHolds(spec, you)
		// A supported predicate is evaluated for every mode. An unsupported
		// one fails closed only for actionTriggerModes; other modes keep
		// firing as they did before the predicate was read at all.
		if (supported && !holds) || (!supported && actionTriggerModes[t.Mode]) {
			return false
		}
	}
	return true
}

// checkDefinedPlayerHolds evaluates the player-state predicate class used by
// event-trigger conditions. isMonarch, with the controller, opponent and any-
// player selectors, is the supported form; supported is false for any other
// predicate (hasInitiative, withMost*, committedCrimeThisTurn, ...).
func (e *Engine) checkDefinedPlayerHolds(spec string, you state.PlayerID) (holds, supported bool) {
	base, property, ok := strings.Cut(strings.TrimSpace(spec), ".")
	if !ok || property != "isMonarch" {
		return false, false
	}
	switch base {
	case "You":
		return e.G.IsMonarch(you), true
	case "Opponent", "Other":
		for _, p := range e.G.AliveFrom(you) {
			if p != you && e.G.IsMonarch(p) {
				return true, true
			}
		}
		return false, true
	case "Player", "Any":
		for _, p := range e.G.AliveFrom(0) {
			if e.G.IsMonarch(p) {
				return true, true
			}
		}
		return false, true
	}
	return false, false
}

// lifeConditionHoldsAs evaluates the LifeTotal$/LifeAmount$ intervening-if.
// The "you" for a You-qualified LifeTotal$ is the caller's chosen player --
// normally the source's controller, but an event-matched delayed trigger
// passes its registration's effect owner instead (see
// checkEventDelayedTriggers).
func (e *Engine) lifeConditionHoldsAs(t cards.Trigger, you state.PlayerID, amount string) bool {
	who := you
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

// presentConditionHoldsAs evaluates the IsPresent$/PresentCompare$
// intervening-if by counting the objects on the battlefield that match the
// spec (relative to the trigger's source and the caller's chosen "you") and
// comparing that count.
func (e *Engine) presentConditionHoldsAs(t cards.Trigger, source state.ObjID, you state.PlayerID, spec, cmp string) bool {
	// PresentDefined$ (Mana Vault's draw-step self-check): the IsPresent$
	// spec is evaluated over the DEFINED set rather than the whole
	// battlefield. "Self" -- the corpus's dominant value -- counts the
	// source object alone when it matches; any other selector falls back to
	// the battlefield-wide count, so an unreadable defined set degrades to
	// the pre-PresentDefined behaviour instead of fail-closing a trigger
	// whose spec the count would otherwise answer.
	if pd := strings.TrimSpace(t.Params["PresentDefined"]); pd != "" {
		if strings.EqualFold(pd, "Self") {
			if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield ||
				!effects.MatchesSpecCtx(e.G, spec, source, e.specCtx(source, you)) {
				return comparePresent(0, cmp)
			}
			return comparePresent(1, cmp)
		}
	}
	// PresentZone$ (Jocasta, Automaton Avenger's "if this card is in your
	// graveyard"): the IsPresent$ spec is counted over the named zone in
	// every living seat's copy of it, in deterministic seat/zone order, the
	// same walk countPresent makes over the battlefield. An unknown zone
	// word fails closed -- a clause this build cannot read must never read
	// as vacuously satisfied.
	if pz := strings.TrimSpace(t.Params["PresentZone"]); pz != "" {
		n, known := e.presentZoneCount(t, spec, source, you)
		if !known {
			return false
		}
		return comparePresent(n, cmp)
	}
	n := e.countPresent(spec, source, you)
	return comparePresent(n, cmp)
}

// presentZoneCount counts spec matches over one zone (PresentZone$'s value)
// across every living seat, the deterministic walk countPresent makes over
// the battlefield. known is false for a zone word this build does not know,
// which every caller fails closed on.
func (e *Engine) presentZoneCount(t cards.Trigger, spec string, source state.ObjID, you state.PlayerID) (int, bool) {
	zone, known := effects.ParseZoneWord(strings.TrimSpace(t.Params["PresentZone"]))
	if !known {
		return 0, false
	}
	n := 0
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(zone, p) {
			if effects.MatchesSpecCtx(e.G, spec, id, e.specCtx(source, you)) {
				n++
			}
		}
	}
	return n, true
}

// countPresentZone is the no-compare IsPresent$ branch's PresentZone$ count:
// an unknown zone word fails closed to "never holds".
func (e *Engine) countPresentZone(t cards.Trigger, spec string, source state.ObjID, you state.PlayerID) bool {
	n, known := e.presentZoneCount(t, spec, source, you)
	return known && n > 0
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

// presentUnionCount counts the DISTINCT battlefield objects matching either
// spec — the IsPresent$+IsPresent2$ union a two-set present clause compares.
func (e *Engine) presentUnionCount(spec, spec2 string, source state.ObjID, you state.PlayerID) int {
	seen := map[state.ObjID]bool{}
	n := 0
	e.forEachObject(func(id state.ObjID) {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			return
		}
		if seen[id] {
			return
		}
		sc := e.specCtx(source, you)
		if effects.MatchesSpecCtx(e.G, spec, id, sc) || effects.MatchesSpecCtx(e.G, spec2, id, sc) {
			seen[id] = true
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
		"trig:ChangesZone", "trig:ChangesZoneAll", "trig:SpellCast", "trig:Attacks", "trig:AttackersDeclaredOneTarget",
		"trig:AttackersDeclared", "trig:AttackerBlocked", "trig:AttackerBlockedByCreature", "trig:Cycled", "trig:CounterAdded", "trig:CounterRemoved",
		"trig:Sacrificed", "trig:Discarded", "trig:CommitCrime", "trig:Taps", "trig:TapsForMana",
		"trig:TokenCreated", "trig:TokenCreatedOnce",
		"trig:DamageDone", "trig:DamageDealtOnce", "trig:DamageDoneOnce", "trig:Drawn", "trig:LifeLost", "trig:LifeLostAll",
		"trig:BecomesTarget", "trig:LandPlayed", "trig:Phase", "trig:Attached", "trig:FlippedCoin",
		"trig:Explores", "trig:Exerted",
		"trig:AbilityCast", "trig:SpellAbilityCast", "trig:Always",
		"repl:Moved",
		// Task 16 keyword triggers, expanded by cards/keywords.go into ordinary
		// ChangesZone / Attacks / SpellCast triggers routed through the modes
		// above: Undying and Evolve are ChangesZone triggers, Exalted is
		// (Alone$) Attacks, Prowess is SpellCast.
		// Persist (CR 702.77) is Undying's mirror: the same ChangesZone
		// dies-trigger shape, reading counters_EQ0_M1M1 off the LKI and
		// returning the permanent with a -1/-1 counter.
		"kw:Undying", "kw:Persist", "kw:Evolve", "kw:Exalted", "kw:Dethrone", "kw:Prowess", "kw:Riot", "kw:Hideaway", "kw:Extort", "kw:Myriad", "kw:Soulbond", "kw:Dredge",
		// Task 17: Storm's expansion (cards/keywords.go) is a SpellCast
		// trigger whose effect is CopySpellAbility -- the expansion existed
		// since Task 11; registering the keyword here completes its
		// semantics now that api:CopySpellAbility is implemented.
		"kw:Storm", "kw:Ward", "kw:Annihilator", "kw:Mobilize",
		// Afflict's expansion (cards/keywords.go) is a become-blocked trigger
		// (trig:AttackerBlocked) whose body drains the defender; the granted
		// (layer-6 AddKeyword$) form is synthesized by
		// checkGrantedAfflictTriggers, the Dethrone precedent.
		"kw:Afflict",
		// Flanking's expansion (cards/keywords.go) is a become-blocked trigger
		// on the new AttackerBlockedByCreature mode (CR 702.25a), one instance
		// per non-flanking blocker, debuffing it -1/-1 until EOT via Pump.
		"kw:Flanking",
		// Afterlife's expansion (cards/keywords.go) is a ChangesZone death
		// trigger whose effect mints the wb_1_1_spirit_flying tokens.
		"kw:Afterlife",
		// Mass effects, extra turns and new-set mechanics (the
		// inbox-engine-gap-mass-turn-new-mechanics ticket):
		//   - trig:UnlockDoor: a Room's unlock trigger (rules/rooms.go),
		//     queued off the DoorUnlock event the unlock activation emits.
		//   - kw:Station: the Spacecraft station activation (rules/station.go).
		//   - kw:Chapter: the Saga mechanic (rules/saga.go).
		//   - kw:Start your engines: the speed mechanic (rules/speed.go).
		//   - stat:Panharmonicon: the trigger-doubling static (rules'
		//     panharmoniconEchoes, consulted in checkFaceTriggers).
		//   - kw:Partner, kw:Partner with and "kw:CARDNAME can be your
		//     commander." are DECK-CONSTRUCTION keywords (CR 903.13a/b, the
		//     CR 903.13c named-pair alias, and CR 903.4-style commander
		//     eligibility): the engine already seats and casts commanders
		//     per Config -- partnerPairOK -> deck.IsPartnerPair checks the
		//     mutual named pair -- and nothing in play reads them, so the
		//     registrations assert the corpus shape is understood, not that
		//     play rules exist for it.
		"trig:UnlockDoor", "kw:Station", "kw:Chapter", "kw:Start your engines",
		"stat:Panharmonicon", "kw:Partner", "kw:Partner with",
		"kw:CARDNAME can be your commander.",
	)
}

// checkGrantedDethroneTriggers synthesizes Dethrone's ordinary attack trigger
// (CR 702.105) for a creature that currently HAS the keyword but does not
// print it: a keyword granted in layer 6 has the same rules text as a printed
// keyword, and keyword expansion only adds triggers for printed K: lines.
// This is deliberately a read-only derived-characteristics check; granting
// and removing the keyword remains entirely in the existing continuous-effect
// system. It must run on the faceMayTrigger early-return path too (a granted
// keyword is independent of printed triggers, the same shape Granted Ward
// is), so it is called once per object before that gate.
func (e *Engine) checkGrantedDethroneTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object) {
	if ev.Kind != events.DeclareAttackers || e.HasKeyword(id, "Dethrone") && f.HasKeyword("Dethrone") {
		return
	}
	if !e.HasKeyword(id, "Dethrone") || f.HasKeyword("Dethrone") {
		return
	}
	t := cards.Trigger{Mode: "Attacks", Params: map[string]string{
		"Mode": "Attacks", "ValidCard": "Card.Self", "Dethrone": "True",
	}, Effect: &cards.SA{Kind: "DB", API: "PutCounter", Params: map[string]string{
		"Defined": "Self", "CounterType": "P1P1", "CounterNum": "1",
	}}}
	if observer.triggerMatches(t, id, ev, objLKI) {
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] < maxTriggerFires {
			e.triggerFireCount[key]++
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{Source: id, Controller: o.Controller, Idx: -1, SA: t.Effect,
				Ctx: effects.Ctx{Source: id, Controller: o.Controller, Remembered: triggerRemembered(ev, id), LKI: objLKI,
					TriggerContext: observer.triggerReferents(t, id, ev, objLKI)}})
		}
	}
}

// checkGrantedAfflictTriggers synthesizes Afflict's become-blocked trigger
// (CR 702.130) for a creature that currently HAS the keyword but does not
// print it: a keyword granted in layer 6 (Lost Monarch of Ifnir's
// "Other Zombies you control have afflict 3") has the same rules text as a
// printed keyword, and the printed K:Afflict expansion (cards/keywords.go)
// only covers printed lines. The synthesized trigger reuses the ordinary
// AttackerBlocked machinery -- triggerModeEvents' DeclareBlockers entry,
// zoneGate/phaseGate, attackerBlockedCandidates' dedup and fire-count bound
// -- so its behaviour is byte-identical to the printed path's for
// ValidCard$ Card.Self: one instance per distinct blocking declaration of
// the granted creature itself, the defender captured as
// TriggerContext.DefendingPlayer for the body's
// Defined$ TriggeredDefendingPlayer. The granted amount is the FIRST
// keyword granted in a layer never matches. The walk does NOT skip a card
// that also prints Afflict: the granted amounts are every Afflict entry in
// the derived list whose exact text no printed line already carries, so a
// card printing K:Afflict:1 that is granted "Afflict 3" fires BOTH (printed
// expansion for 1, this walk for 3), while a grant identical to a printed
// line is skipped -- the printed expansion already owns it and the two are
// indistinguishable as strings (a Monarch carrying another Monarch's
// identical grant keeps its one printed instance; the conservative choice).
// Every distinct grant queues its own instance, so two Monarchs on the
// battlefield stack two afflict-3 instances on the same Zombie. Like
// Dethrone's synthesis this is a read-only derived-characteristics check;
// granting stays in the continuous-effect system.
func (e *Engine) checkGrantedAfflictTriggers(id state.ObjID, o *state.Object, f *cards.Face, ev events.Event) {
	if ev.Kind != events.DeclareBlockers {
		return
	}
	if !e.HasKeyword(id, "Afflict") {
		return
	}
	printed := map[string]bool{}
	for _, k := range f.Keywords {
		printed[strings.ToLower(k)] = true
	}
	var amounts []int
	for _, k := range e.Derived(id).Keywords {
		if !strings.EqualFold(cards.KeywordHead(k), "Afflict") || printed[strings.ToLower(k)] {
			continue
		}
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		amount, err := strconv.Atoi(param)
		if err != nil || amount <= 0 {
			continue // a malformed or non-positive grant delivers no trigger.
		}
		amounts = append(amounts, amount)
	}
	if len(amounts) == 0 {
		return
	}
	t := cards.Trigger{Mode: "AttackerBlocked", Params: map[string]string{
		"Mode": "AttackerBlocked", "ValidCard": "Card.Self",
	}}
	if !e.zoneGate(t, id, ev) || !e.phaseGate(t) {
		return
	}
	key := triggerKey{Source: id, Idx: -1}
	if e.triggerFireCount == nil {
		e.triggerFireCount = map[triggerKey]int32{}
	}
	if e.triggerFireCount[key] >= maxTriggerFires {
		return // cascade bound: see maxTriggerFires.
	}
	pt := func(p state.PlayerID) state.Target { return state.Target{Player: p, IsPlayer: true} }
	for _, aid := range e.attackerBlockedCandidates(t, id, ev) {
		// ValidCard$ Card.Self admits only the granted creature itself;
		// attackerBlockedCandidates still returns other attackers this event
		// blocked, so skip anything that is not the source.
		if aid != id {
			continue
		}
		defender := pt(0)
		if ao := e.G.Obj(aid); ao != nil {
			defender = pt(ao.Attacking)
		}
		for _, amount := range amounts {
			e.triggerFireCount[key]++
			e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
				Source:     id,
				Controller: o.Controller,
				// The Ward shape: the body rides the push's __kwAfflict:
				// payload for events.Apply to rebuild structurally -- a raw
				// SA cannot cross the log, and the TriggerPush -1 index
				// sentinel is Dethrone's own.
				Afflict: strconv.Itoa(amount),
				Ctx: effects.Ctx{
					Source:     id,
					Controller: o.Controller,
					Remembered: []state.Target{{Obj: aid}},
					Captured:   []state.Target{{Obj: aid}},
					TriggerContext: effects.TriggerContext{
						TriggerCard:     aid,
						TriggerSource:   aid,
						AttackingPlayer: pt(e.controllerOf(aid)),
						DefendingPlayer: defender,
					},
				},
			})
		}
	}
}

func (e *Engine) checkGrantedWardTriggers(observer *Engine, id state.ObjID, o *state.Object, f *cards.Face, ev events.Event, objLKI *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool) {
	if e.finishingLifeLossBatch || e.lifeLossBatchDepth > 0 {
		return
	}
	// Every synthesized Ward uses BecomesTarget + Card.Self. Apply that
	// matcher's cheap gates before deriving characteristics for every object
	// in every zone. Keep the caller's visitation/queue order unchanged.
	if ev.Kind != events.TargetsChosen || !slices.Contains(ev.IDs, id) {
		return
	}
	printed := map[string]bool{}
	if !e.faceDownPrintedHides(o) {
		// While the object is face down its printed face does not exist
		// (CR 708.8): a cloaked card whose real face prints Ward must not
		// suppress its own cloak-ward -- the derived list is the only
		// keyword source on this path.
		for _, k := range f.Keywords {
			printed[strings.ToLower(k)] = true
		}
	}
	for _, k := range observer.Derived(id).Keywords {
		if printed[strings.ToLower(k)] {
			continue
		}
		param := ""
		if head := cards.KeywordHead(k); !strings.EqualFold(head, "Ward") {
			continue
		} else if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		if param == "" {
			continue
		}
		t := cards.Trigger{Mode: "BecomesTarget",
			Params: map[string]string{"ValidTarget": "Card.Self", "Ward": "True", "TriggerDescription": "Ward"},
			Effect: &cards.SA{Kind: "DB", API: "Ward",
				Params: map[string]string{"UnlessCost": param, "TriggerDescription": "Ward"}}}
		if !observer.triggerMatches(t, id, ev, objLKI) {
			continue
		}
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     id,
			Controller: o.Controller,
			Ward:       param,
			Ctx: effects.Ctx{
				Source:         id,
				Controller:     o.Controller,
				Remembered:     triggerRemembered(ev, id),
				Captured:       triggerRemembered(ev, id),
				LKI:            objLKI,
				LKIPower:       lkiPower,
				LKIToughness:   lkiToughness,
				LKIPTValid:     objLKI != nil && lkiPTValid,
				TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
			},
		})
	}
}

// checkGrantedStaticTriggers queues the triggered abilities a live static
// grant (AddTrigger$ on a Mode$ Continuous static, e.g. Hearthhull's
// "STATION 8+ Whenever you sacrifice a land") gives this object: the
// checkGrantedWard/checkGrantedDethrone precedent -- a granted triggered
// ability has the same rules text a printed one would, and the face walk
// above only ever scans printed T: lines. The grants live on the memoised
// static scan (active()); each Affected$-matched grant is matched against
// the event exactly as a printed trigger would be (triggerMatches, which
// resolves the granted body's own TriggerZones$/ValidPlayer$/Phase$ clauses)
// and queued with its SVar-resolved SA -- the pendingTrigger shape the
// granted-keyword paths use, pushed through events.GrantTriggerPush.
//
// The queue gate is the live==replay contract: the stack object is minted
// inside events.Apply, which can only resolve the Execute$ body from the
// AFFECTED object's own SVar table, so the walk links the effect from that
// same table (the resolveSVarAcrossFaces walk mirrored in
// grantedTriggerExecute) and a grant whose body it cannot produce never
// queues -- the conservative direction, matching the replayable-log
// invariant rather than minting an ability a replay cannot rebuild. A
// self-grant (Hearthhull) trivially satisfies it; the cross-object
// aura-grants-its-own-SVar shape fails closed here.
//
// Fire-count: like Ward and Dethrone, every granted trigger shares the
// granted slot's triggerKey (Source, Idx -1) -- the cascade bound only, not
// once-per-turn memory, and maxTriggerFires (256) is generous enough that
// the sharing cannot starve a legitimate fire. Like those two the walk is
// deliberately a read over active()'s sorted slice, never a map: the queue
// order stays the scan's deterministic order.
func (e *Engine) checkGrantedStaticTriggersUsing(observer *Engine, statics []ContinuousEffect, id state.ObjID, o *state.Object, ev events.Event, objLKI *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool) {
	if e.finishingLifeLossBatch || e.lifeLossBatchDepth > 0 {
		return
	}
	for i := range statics {
		ce := &statics[i]
		if ce.AddTrigger == nil {
			continue
		}
		if !effects.MatchesSpecFrom(observer.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		t := *ce.AddTrigger
		// The live==replay gate: link the Execute$ body exactly the way
		// events.Apply will (the affected object's own table); a body it
		// cannot resolve never queues, and a same-named body it CAN resolve
		// is by construction the same body a replay would resolve.
		if t.Effect = grantedTriggerExecute(o, t.Params["Execute"]); t.Effect == nil {
			continue
		}
		// CR 603.8's outstanding-instance latch, mirrored from the face walk
		// (a state trigger already queued or on the stack does not re-fire).
		if t.Mode == "Always" && e.stateTriggerOutstanding(id, -1) {
			continue
		}
		// LifeLostAll is evaluated once at the end of a simultaneous
		// life-loss batch, exactly as the face walk scopes it.
		if t.Mode == "LifeLostAll" && e.lifeLossBatchDepth > 0 && !e.finishingLifeLossBatch {
			continue
		}
		if !observer.triggerMatches(t, id, ev, objLKI) {
			continue
		}
		key := triggerKey{Source: id, Idx: -1}
		if e.triggerFireCount == nil {
			e.triggerFireCount = map[triggerKey]int32{}
		}
		if e.triggerFireCount[key] >= maxTriggerFires {
			continue // cascade bound: see maxTriggerFires.
		}
		e.triggerFireCount[key]++
		e.pendingTriggers = append(e.pendingTriggers, pendingTrigger{
			Source:     id,
			Controller: o.Controller,
			Idx:        -1,
			SA:         t.Effect,
			Granted:    true,
			Execute:    t.Params["Execute"],
			Ctx: effects.Ctx{
				Source:         id,
				Controller:     o.Controller,
				Remembered:     triggerRemembered(ev, id),
				LKI:            objLKI,
				LKIPower:       lkiPower,
				LKIToughness:   lkiToughness,
				LKIPTValid:     objLKI != nil && lkiPTValid,
				TriggerContext: observer.triggerReferents(t, id, ev, objLKI),
			},
		})
	}
}

// grantedTriggerExecute mirrors events.Apply's GrantTriggerPush resolution
// (the resolveSVarAcrossFaces walk): the granted body's Execute$ name is
// resolved against the AFFECTED object's own SVar table -- current face
// first, then every other face -- so the live queue links exactly the body a
// replayed log will. nil when no face resolves it.
func grantedTriggerExecute(o *state.Object, execute string) *cards.SA {
	if execute == "" {
		return nil
	}
	if f := o.Face(); f != nil {
		if sa := cards.ResolveSVar(f.SVars, execute); sa != nil {
			return sa
		}
	}
	if o.Card == nil {
		return nil
	}
	for _, cf := range o.Card.Faces {
		if sa := cards.ResolveSVar(cf.SVars, execute); sa != nil {
			return sa
		}
	}
	return nil
}

// triggerCastAlt is one surviving ValidCard$ alternative of a Mode$
// SpellCast trigger after the trigger-side self-cast exclusion was applied:
// spec is the token-stripped filter text, exclSelf whether the alternative
// carried the bare !CastSaSource token — its ActivatorThisTurnCastEach$
// tally must also skip the trigger source's own printed-name casts.
type triggerCastAlt struct {
	spec     string
	exclSelf bool
}

// triggerCastAlternatives splits a Mode$ SpellCast trigger's ValidCard$ into
// its comma alternatives and applies the trigger-side bare !CastSaSource
// reading (task castprov2, Alania, Divergent Storm — measured: exactly 1
// trigger-side line in the corpus): an alternative carrying the token names
// "a spell whose PRINTED NAME is not the trigger SOURCE's printed name" (the
// oracle's "other than NICKNAME"; Forge has no trigger-side CastSaSource
// grammar precedent in this corpus, so this reading is a documented
// decision, not a discovered fact). The cast card's printed name equal to
// the source's drops the alternative (a second Alania cast fails the Otter
// alternative and does not trigger); the token is otherwise stripped and the
// alternative kept for the ordinary filter. ok is false when no alternative
// survives.
func (e *Engine) triggerCastAlternatives(rawSpec string, source, castObj state.ObjID) ([]triggerCastAlt, bool) {
	spec := spellCastPermanentSpec(rawSpec)
	sourceName, castName := "", ""
	if o := e.G.Obj(source); o != nil && o.Face() != nil {
		sourceName = o.Face().Name
	}
	if o := e.G.Obj(castObj); o != nil && o.Face() != nil {
		castName = o.Face().Name
	}
	isSelf := sourceName != "" && castName == sourceName
	var alts []triggerCastAlt
	for alt := range effects.FilterAlternatives(spec) {
		s, had := effects.StripPredicateToken(alt, "!CastSaSource")
		if had && isSelf {
			continue
		}
		alts = append(alts, triggerCastAlt{spec: s, exclSelf: had})
	}
	if len(alts) == 0 {
		return nil, false
	}
	return alts, true
}

// spellsCastThisTurnByMatching counts player p's spells cast this turn whose
// object matches ONE alternative spec, with the trigger-side NICKNAME
// exclusion applied to the tally when that alternative carried the
// !CastSaSource token (Alania's "the first Otter spell other than Alania":
// the source's own casts do not count toward that alternative's first).
// The current cast is INCLUDED — it is already in the log when the deferred
// trigger fires, and the oracle's "first ... you've cast this turn" counts
// it (EQ1 = this cast is the first).
func (e *Engine) spellsCastThisTurnByMatching(p state.PlayerID, spec string, exclSelf bool, source state.ObjID) int {
	selfName := ""
	if exclSelf {
		if o := e.G.Obj(source); o != nil && o.Face() != nil {
			selfName = o.Face().Name
		}
	}
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind != events.PutOnStack || ev.Player != p {
			continue
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		if selfName != "" && o.Face().Name == selfName {
			continue
		}
		if effects.MatchesSpecFrom(e.G, spec, ev.Obj, p, ev.Obj) {
			n++
		}
	}
	return n
}
