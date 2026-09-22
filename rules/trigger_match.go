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
	// Merged marks a mutated pile's under-card trigger (CR 702.140d): like
	// a delayed trigger its Ability is the Execute$ SVar-named body, but the
	// push must resolve that name against the UNDER-CARD's own face, never
	// the pile's top face (whose same-named SVar -- Forge's canonical
	// TrigToken -- would otherwise steal the body). The value is the
	// under-card's pile index PLUS ONE, the same encoding triggerFace.merged
	// uses, so the zero value always means "not a merged trigger". It rides
	// the MergedTriggerPush event as Amount (minus one). It and Delayed are
	// never both set.
	Merged int
	// Granted marks a static-grant's trigger (AddTrigger$ on a Mode$
	// Continuous static, e.g. Hearthhull's "STATION 8+ Whenever you sacrifice
	// a land"): like a delayed trigger its Ability is an SVar-named body (the
	// Execute$ name rides the GrantTriggerPush event for events.Apply to
	// resolve from the GRANTOR's SVar table), but unlike a delayed
	// registration nothing is consumed -- the grant lives exactly as long as
	// its granting static. Grantor is the object carrying the printed static
	// (0 = the self-grant shape, where grantor == recipient); it rides the
	// event's Amount so Apply resolves the Execute$ body from the same table
	// this walk linked it from.
	Granted bool
	Grantor state.ObjID
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
	// Conspire is a GRANTED conspire keyword (a layer-6 AddKeyword$
	// Conspire -- Wort, the Raidmother's "each red or green instant or
	// sorcery spell you cast has conspire", Raiding Schemes' noncreature
	// arm): the same shape as Ward/Afflict -- the queue carries no
	// parameter (the copy trigger has none) and the drain pushes a
	// KeywordTriggerPush whose __kwConspire payload events.Apply rebuilds
	// into the same DB$ CopySpellAbility body the printed K:Conspire
	// expansion carries, with the cast spell riding IDs as Remembered.
	// Idx and SA are unset for it.
	Conspire bool
	// Cascade is a printed-or-granted cascade keyword (CR 702.85, task
	// cascade1): the queue carries no parameter (the trigger body is the
	// same DB$ Cascade body whichever route granted the keyword) and the
	// drain pushes a KeywordTriggerPush whose __kwCascade payload
	// events.Apply rebuilds structurally -- the Ward shape. The trigger's
	// Source is the CAST SPELL (the stack object), whose face's mana value
	// the effect reads at resolution. Idx and SA are unset for it.
	Cascade bool
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

// combatFires is one T: line's once-per-combat latch stamp
// (Engine.unblockedOnceFired): the (turn, combat-of-that-turn) pair that
// uniquely names the combat the line last fired in. A new turn or a new
// combat phase within it changes the stamp and re-arms the trigger.
type combatFires struct {
	Turn   int32
	Combat int32
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
	// LifeGained joins them for the same reason: it is an event mode
	// registered from the start (lifeGainedMatches over events.LifeChange,
	// the api:RemoveCounter ticket's Prize Pig pin), so the trigger-level
	// parameters Forge scopes to every event mode -- PlayerTurn$ (5 corpus
	// lines: Vampire Scrivener, Wax//Wane Witness, Moonstone Harbinger,
	// Cat Collector), ActivationLimit$ (2 lines) and an unevaluable
	// CheckDefinedPlayer$ predicate failing closed -- apply. lifeGainedMatches
	// itself reads FirstTime$ (8 lines over 7 files: Attended Healer,
	// Deathless Knight, Vanguard Seraph, Gourmand's Talent, ...), the
	// once-per-turn latch lifeLostMatches implements without the map.
	// LifeGained joins them for the same reason: it is an event mode
	// registered from the start (lifeGainedMatches over events.LifeChange,
	// the api:RemoveCounter ticket's Prize Pig pin), so the trigger-level
	// parameters Forge scopes to every event mode -- PlayerTurn$ (5 corpus
	// lines: Vampire Scrivener, Wax//Wane Witness, Moonstone Harbinger,
	// Cat Collector), ActivationLimit$ (2 lines) and an unevaluable
	// CheckDefinedPlayer$ predicate failing closed -- apply. lifeGainedMatches
	// itself reads FirstTime$ (8 lines over 7 files: Attended Healer,
	// Deathless Knight, Vanguard Seraph, Gourmand's Talent, ...), the
	// once-per-turn latch lifeLostMatches implements without the map.
	"LifeGained": true,
	// Discover and SeekAll join them: both are event modes registered with
	// their own marker Kinds (events.Discover/events.Seek, task trigdisc1)
	// whose corpus carriers carry the trigger-level parameters Forge scopes
	// to every event mode -- Curator of Sun's Creation's ActivationLimit$ 1
	// on its Discover line ("This ability triggers only once each turn")
	// and Lurker in the Deep's PlayerTurn$ True on its SeekAll line -- so
	// both gates must apply from day one.
	"Discover": true, "SeekAll": true,
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
	if ev.Kind == events.PutOnStack || ev.Kind == events.MoveZone {
		e.checkEventDelayedTriggers(ev, lki)
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
		e.checkBlocksTriggers(ev)
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
		if !o.Unlocked && len(o.MergedCards) == 0 && !e.objectFaceMayTrigger(id, o.FaceIdx, f, ev.Kind) {
			if grantedKeywordTriggerEvent(ev.Kind) {
				switch ev.Kind {
				case events.TargetsChosen:
					e.checkGrantedWardTriggers(observer, id, o, f, ev, objLKI, lkiPower, lkiToughness, lkiPTValid)
				case events.DeclareAttackers:
					e.checkGrantedDethroneTriggers(observer, id, o, f, ev, objLKI)
				case events.DeclareBlockers:
					e.checkGrantedAfflictTriggers(id, o, f, ev)
				case events.PutOnStack:
					e.checkGrantedConspireTriggers(observer, id, o, f, ev, objLKI)
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
		walk := faces[:n]
		if len(o.MergedCards) > 0 {
			// CR 702.140d: a mutated permanent has all abilities of the cards
			// beneath its top card, so their printed triggers must be walked
			// too. triggerFacesWithMerged marks them active=false, which routes
			// each through the by-name push (the Room alternate-face path) --
			// TriggerPush can only name a trigger INDEX into the top face, so an
			// under-card trigger must resolve its Execute$ SVar by name instead.
			// Ordinary (unmutated) objects keep the allocation-free [2]array
			// path above.
			walk = triggerFacesWithMerged(o, faces[:n])
		}
		for _, fc := range walk {
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
				//
				// A MERGED under-card face (CR 702.140d) must NOT take that
				// by-name path: resolveSVarAcrossFaces resolves top-face-first,
				// so the pile's top card -- which can be ANY creature, and whose
				// own token triggers canonically name their SVar TrigToken --
				// would steal the under-card's body whenever both faces define
				// the same name (Cubwarden mutated under Everquill Phoenix: the
				// under-card's "create two Cats" resolved to the top Phoenix's
				// "create a Feather"). MergedTriggerPush instead carries the
				// under-card's pile index so events.Apply resolves the name
				// against THAT face's own SVar table.
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
				switch {
				case fc.merged > 0:
					pt.Merged = fc.merged
					pt.Execute = t.Params["Execute"]
				case !fc.active:
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
		// A granted Conspire must fire even when the object's own printed
		// triggers are live for this event (a spell with its own cast trigger
		// carrying a Conspire grant) -- the early-return path above reaches this
		// object through checkGrantedConspireTriggers's own call.
		e.checkGrantedConspireTriggers(observer, id, o, f, ev, objLKI)
	})
	for _, n := range phaseNotes {
		e.emit(events.Event{Kind: events.Note, Obj: n.id,
			Text: "Phase$ " + n.spec + " names no engine step; the trigger never fires"})
	}
}

// triggerFace is one face's trigger walk: its printed index keys fire-count
// memory, and active says whether TriggerPush can re-derive it through the
// object's current Face() (the other unlocked Room face must use a delayed
// shape because Apply cannot select it). merged is 0 for every real face;
// a MERGED under-card face carries its pile index PLUS ONE so the zero
// value can never alias pile index 0 (a forgotten field must always mean
// "not merged", never "the first under-card").
type triggerFace struct {
	face    *cards.Face
	faceIdx uint8
	active  bool
	merged  int
}

// roomTriggerFaces returns the faces whose Triggers a scan walks for object
// o: its cast face always, plus the other face once unlocked (CR 309.6).
// FaceIdx need not be zero: CR 309.4b permits casting either Room door.
func roomTriggerFaces(o *state.Object, active *cards.Face) ([2]triggerFace, int) {
	// At most two faces, returned by value so the ordinary single-face walk
	// never allocates a backing slice per object per event. merged is left 0
	// ("not merged") on both.
	out := [2]triggerFace{{face: active, faceIdx: o.FaceIdx, active: true}}
	if o.Unlocked && isRoom(o) && len(o.Card.Faces) == 2 && int(o.FaceIdx) < len(o.Card.Faces) {
		other := uint8(1 - int(o.FaceIdx))
		out[1] = triggerFace{face: o.Card.Faces[other], faceIdx: other}
		return out, 2
	}
	return out, 1
}

// triggerFacesWithMerged extends the room faces with every card stacked
// beneath a mutated permanent's top card (CR 702.140d's "all abilities of
// the cards beneath it"). It allocates only for a mutated pile, so the
// ordinary trigger scan keeps roomTriggerFaces' allocation-free fast path.
// A merged face is marked active=false AND carries its pile index (+1, the
// encoding triggerFace.merged documents): the queue routes it through the
// MergedTriggerPush push, which carries the pile index so events.Apply
// resolves the under-card's Execute$ SVar against THAT face's table -- the
// top face's same-named SVar must never steal the body. faceIdx 2+i sits
// outside the two real card-face slots so its triggerKey/fire-count entries
// never collide with the top card's; a pile big enough to overflow the byte
// (254+ merged cards -- no real game reaches it; each merged card is one
// full mutate cast and resolution) stops being walked rather than colliding.
func triggerFacesWithMerged(o *state.Object, base []triggerFace) []triggerFace {
	out := make([]triggerFace, 0, len(base)+len(o.MergedCards))
	out = append(out, base...)
	for i := range o.MergedCards {
		if 2+i > 255 {
			break
		}
		if face := o.MergedFaceAt(i); face != nil {
			out = append(out, triggerFace{face: face, faceIdx: uint8(2 + i), merged: i + 1})
		}
	}
	return out
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

// trigMatcher answers whether one trigger fires for ev. lki is the event's LKI
// snapshot (the moving object as it was a moment ago); a matcher that does not
// need it ignores the parameter. The uniform signature is what lets every mode
// live behind one table.
type trigMatcher func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool

// trigMatchers maps Mode$ to its matcher. A mode with no entry never fires,
// which is exactly what the switch this replaced did by falling off its end
// with matched still false -- there was no default arm, and there must not be
// one now.
var trigMatchers = map[string]trigMatcher{}

// registerTrigMatcher installs fn for each named mode. Called from init() in
// the per-mode trigmatch_*.go files.
//
// A duplicate registration panics rather than silently replacing. The whole
// point of the split is that many tickets edit different files at once; two
// files claiming one mode is the merge accident that costs, so it must be loud
// at startup and not a matcher that quietly stopped being reached.
func registerTrigMatcher(fn trigMatcher, modes ...string) {
	for _, mode := range modes {
		if _, dup := trigMatchers[mode]; dup {
			panic("rules: duplicate trigger matcher registered for Mode$ " + mode)
		}
		trigMatchers[mode] = fn
	}
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
	// Per-mode dispatch: see trigMatchers. A mode with no registered
	// matcher never fires, which is what the old switch did by falling
	// off its end with matched still false.
	var matched bool
	if fn := trigMatchers[t.Mode]; fn != nil {
		matched = fn(e, t, source, ev, lki)
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

func init() {
	// CR 603.8 state trigger: the event under test is irrelevant; the trigger
	// fires when its condition holds (see triggerConditionHolds) and no
	// instance is outstanding (the checkTriggers latch).
	registerTrigMatcher(func(*Engine, cards.Trigger, state.ObjID, events.Event, *state.Object) bool {
		return true
	}, "Always")

	effects.RegisterNonAPI(
		"trig:ChangesZone", "trig:ChangesZoneAll", "trig:SpellCast", "trig:Attacks", "trig:AttackersDeclaredOneTarget",
		"trig:AttackersDeclared", "trig:AttackerBlocked", "trig:AttackerBlockedByCreature", "trig:AttackerUnblockedOnce", "trig:Blocks", "trig:Cycled", "trig:CounterAdded", "trig:CounterAddedOnce", "trig:CounterRemoved",
		"trig:Sacrificed", "trig:Discarded", "trig:CommitCrime", "trig:Taps", "trig:TapsForMana",
		"trig:TokenCreated", "trig:TokenCreatedOnce",
		"trig:DamageDone", "trig:DamageDealtOnce", "trig:DamageDoneOnce", "trig:Drawn", "trig:LifeLost", "trig:LifeLostAll",
		"trig:LifeGained",
		"trig:BecomesTarget", "trig:LandPlayed", "trig:Phase", "trig:Attached", "trig:FlippedCoin",
		"trig:Vote",
		"trig:Explores", "trig:Exerted", "trig:Investigated",
		"trig:Connives",
		"trig:Discover", "trig:SeekAll",
		"trig:AbilityCast", "trig:SpellAbilityCast", "trig:Always",
		// The cast-or-copy pair: SpellCopy matches a copy put on the stack and
		// SpellCastOrCopy matches either half (magecraft). Both are matched
		// above and gated in trigger_eligibility.go -- proved by
		// TestMagecraftSpellCastOrCopyFiresOnACast, ...FiresOnACopy and
		// TestSpellCopyPrimarySilentOnAPlainCast. Implemented but never
		// registered, so the ratchet read them as gaps.
		"trig:SpellCopy", "trig:SpellCastOrCopy",
		// The rest of the same registration class, found by sweeping the
		// modes triggerModeEvents maps against this list: each matches and
		// carries its own proof file, and none was registered.
		// trig:DamagePreventedOnce (damagePreventedMatches,
		// damage_prevented_once_test.go: TestDamagePreventedOnceFiresPer
		// Prevention) and trig:RingTemptsYou (ring_test.go:
		// TestCR701RingTemptsYouCallOfTheRingUpkeep).
		"trig:DamagePreventedOnce", "trig:RingTemptsYou",
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
