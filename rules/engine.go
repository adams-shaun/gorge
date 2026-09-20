// Package rules is the authoritative rules engine: turn structure, priority,
// the stack, combat and state-based actions. It owns the only Game instance a
// match has and mutates it exclusively through events.Emit.
//
// The event log is NOT a complete match description by itself. Genesis --
// state.NewGame and the initial AddObject calls that build each player's
// deck into the object arena, both in New below -- runs before the log
// exists and legitimately bypasses events. Replaying L.Events alone
// reconstructs everything that happens from that point on (turn structure,
// zone moves, life, damage, priority and so on), but it can never recover
// deck contents or player names: those are never logged. A faithful replay
// needs the original Config together with the log, not the log alone.
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/deck"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Format is the game's construction format. FormatConstructed is the zero
// value -- every existing Config that never sets it (all today's fixtures
// and every non-Commander game) is unchanged.
type Format int

const (
	FormatConstructed Format = iota
	FormatCommander
)

type Config struct {
	Seed  uint64
	Names []string
	// PlayerNames is a per-seat display name independent of the deck
	// (the wire's PlayerView.Name carries it; Names is the deck identity
	// the engine uses in event text, which is replay-hashed). nil or a
	// short slice falls back to Names, so every Config that never sets it
	// behaves byte-identically to today.
	PlayerNames []string
	Decks       [][]*cards.Card
	// Format names the construction format. Zero means Constructed; the other
	// tasks in the Commander milestone (the tax, CR 903.9, commander damage)
	// read it. This task is plumbing: it reads Commanders and StartingLife
	// only.
	Format Format
	// StartingLife is a game's opening life total. 0 (the zero value) means
	// the existing 20, so every Config that never sets it observes exactly
	// what it always did.
	StartingLife int32
	// Commanders holds, for each seat i, the indices into Decks[i] that are
	// that seat's commanders; nil or absent means none. Genesis places those
	// objects in the command zone instead of the library. An index out of
	// range for its deck is a config error degraded the same way New degrades
	// more decks than seats: the offending entry is skipped, not a crash.
	Commanders [][]int
	// Mulligans is the number of London mulligans each player may take in the
	// pre-game round between the opening deal and turn 1. 0 (the zero value)
	// skips the round entirely, so every Config that never sets it is
	// unchanged (all standalone fixture Configs). It sits in the same Config
	// replay is handed, so a replay reproduces the round (Ruling R-8.4): the
	// acceptance config sets it so the 12-deck suite exercises keep/mulligan
	// and bottoming.
	Mulligans int
	// Tokens is the token definitions the decks in this match can create --
	// cards.Registry.Tokens. Copied onto Game.Tokens in New so
	// events.Apply's TokenCreate case has something to mint from. Replay
	// must pass the same table a live match's Config did.
	Tokens map[string]*cards.Card
	// LoopGuard, when non-nil, overrides the livelock watcher's thresholds
	// for this game (rules/livelock.go): how many consecutive events a
	// repeating cycle must run before the engine aborts with a
	// *LivelockError, the longest cycle tracked, and the no-progress
	// runaway backstop. nil (the zero value every existing Config has) is
	// the defaults, so every game that never sets it is byte-identical to
	// an un-watched one. LoopGuard.Disabled (explicitly set) turns the
	// watcher off entirely -- the embedder's own opt-out for a supervised
	// non-terminating game; the host sets it whenever its
	// MaxDecisionsPerTurn opt-out is in force. The watcher is pure
	// observation either way: it emits no event and holds no state the
	// engine reads.
	LoopGuard *LoopGuard
}

type triggerObjectLKI struct {
	object           *state.Object
	power, toughness int32
	ptValid          bool
}

type Engine struct {
	G            *state.Game
	L            *events.Log
	compiledText *compiledText

	// turnsTaken caches the TurnChange census used by Count$TurnsThisGame.
	// turnsTakenEpoch is the log length represented by the cache; emit advances
	// both together, while an Engine assembled around an existing log lazily
	// rebuilds on its first query.
	turnsTaken      []int32
	turnsTakenEpoch int

	// format is the construction format New was configured with (Config.
	// Format). It is the explicit gate the Commander rules (the tax, CR
	// 903.9, commander damage) check -- "in a non-Commander game none of
	// this runs at all" -- rather than inferring the format from the
	// incidental shape of the zones. It is read long after New returns: at
	// cast time for the CR 903.8 tax, at combat-damage time and in the
	// state-based-action pass for CR 903.10. Plain Format value; Clone
	// copies it so a cloned Commander engine still gates its command-zone
	// rules and keeps its commander-damage loss condition.
	format Format

	rng     *rng
	pending *decision.Decision

	// deferGameOver is true only while New processes the opening deal. A
	// library-empty draw still emits PlayerLost and runs every other SBA, but
	// checkGameOver waits until New has recorded the CR 103.1 toss. That keeps
	// GameOver as the final event of a terminal genesis burst (the host's
	// persisted-boundary contract) and lets an all-undersized opening deal
	// reach the truthful no-survivor draw instead of accidentally crowning an
	// undealt short deck.
	deferGameOver bool

	// continuous holds every registered continuous effect, live or expired.
	// The layer system (layers.go) is the only reader and writer.
	continuous []ContinuousEffect
	// controlGrants holds the GainControl effects that can still end (see
	// rules/control.go). It is engine continuation state only; every take and
	// return is a ControlChange event, so the log alone rebuilds Game state.
	controlGrants []controlGrant
	// expiringControl guards expireControl against re-entry through the
	// ControlChange events it emits.
	expiringControl bool
	// reconcilingControlStatics guards reconcileControlStatics (rules/
	// control_static.go) against re-entry through the ControlChange events
	// IT emits; the same intent-boundary discipline as expiringControl.
	reconcilingControlStatics bool

	// pregame is true while the London mulligan round runs, between the
	// opening deal and turn 1. Config.Mulligans > 0 sets it in New; step()
	// dispatches to stepPregame (rules/mulligan.go) while it is true, and the
	// round's end clears it and hands to beginTurn. Bool field, so Clone
	// copies it like every other value field.
	pregame bool
	// mulligan is the round's plain-value state (rules/mulligan.go) -- seats,
	// kept/taken counts and the phase cursor. Never a closure, so Clone copies
	// it like cast/choosing.
	mulligan mulliganRound
	// opening is the optional opening-hand effects round, after the London
	// mulligan round (a Gemstone Caverns may not be used from a hand its owner
	// later mulliganed away) and before turn one. It holds only object IDs and parsed SVar names, so replay and
	// Clone reproduce the same pregame choices without ambient state.
	// Impatient Iguana's accepted BecomeStartingPlayer$ Reveal resolves here
	// and folds the designation into state.Game through events.StartingPlayer
	// Change (effects/cardflow.go), so Count$StartingPlayer and the view's
	// pregame projection read it before turn one.
	opening openingRound
	// blockerRound is the declare-blockers step's per-defender cursor
	// (rules/combat.go, Task m34): an attack may be split across several
	// defending players, and each declares its own blocks, one KBlockers
	// decision at a time. Plain-value state (a defender list plus an index),
	// never a closure, so Clone copies it like the mulligan round.
	blockerRound blockerRound

	// exertAskState is the declare-attackers exert election's resumable
	// state (rules/combat.go, task exert1): the deterministic offer list
	// (attacking creatures carrying an offerable stat:OptionalAttackCost
	// static, in declaration option order) plus the cursor of the ask
	// currently outstanding. Plain-value state, so Clone copies it like
	// blockerRound; a log-driven replay re-derives the same list when it
	// re-runs the recorded KAttackers answer through handleAttackers.
	exertAskState exertAsk

	// stationing is the spacecraft a pending Station tap pick (rules/
	// station.go) belongs to: the "station" priority option's object, held
	// across the KChoose so the answer's charge counters land on the right
	// permanent. Plain value, so Clone copies it like blockerRound; zero
	// whenever no station ask is outstanding.
	stationing state.ObjID

	// combatRound is the combat damage step's continuation state
	// (rules/combat.go, Task jj-cmb): which damage passes are done, and any
	// controller damage-division choices still being collected or awaiting an
	// answer (CR 510.1c multi-block division, CR 510.3/4 double-strike).
	// Plain-value data (slices plus scalars), never a closure, so Clone
	// copies it like blockerRound and a log-driven replay re-derives the same
	// branch. Zero whenever the combat damage step is not in progress.
	combatRound combatRound

	// staticContinuous memoizes the S:Mode$ Continuous statics on battlefield
	// permanents (layers.go's staticEffects), keyed on staticEpoch. staticEpoch
	// is the log length at the last build; the memo refreshes once per emitted
	// event rather than once per Derived() call, because most events
	// (Priority/Damage/Mana) leave the battlefield permanent set untouched
	// while Derived is the hottest path in a turn (every legal action, combat
	// step and trigger predicate reads it). Clone() leaves both fields zero, so
	// a cloned engine rebuilds the memo on its first Derived -- staticEffects
	// is a pure function of the current board, so the rebuilt result is
	// identical and deterministic. Rebuilds reuse the outer slice's capacity,
	// clearing obsolete slots when it shrinks, but never reuse the nested
	// keyword/type slices. activeBuf copies the effect values into distinct
	// storage before sorting; neither buffer may alias a clone's scratch.
	staticContinuous []ContinuousEffect
	staticEpoch      int

	// staticQueueBuf is staticEffects' AddStaticAbility$ work queue's reused
	// backing array: truncated to zero at every scan, grown only when a
	// static-grant fires (the warm-rescan allocation budget,
	// static_effects_buffer_test, is why it is reused rather than re-made).
	// Per-scan scratch, never cloned: a clone starts nil and grows its own.
	staticQueueBuf []staticWork

	// activeBuf is the cached, fully CR-613-sorted result of layers.go's
	// active(), the effect list every Derived() call ranges over for every
	// object of every board build and projection. Rebuilding that sorted list
	// once per emitted event instead of once per Derived() call is the whole
	// saving here -- active() used to allocate a fresh slice per call, and
	// Derived is the hottest path in a turn. The cache key is the pair
	// (activeEpoch, activeVersion): activeEpoch is the log length at the last
	// build and activeVersion the continuousVersion (bumped by AddContinuous
	// and EndOfTurnCleanup), because the effect list is a pure function of the
	// current board plus e.continuous, and those are exactly the two inputs
	// the key captures -- every board change moves the log head (emit), and
	// e.continuous changes through exactly the two mutators above. activeDepth
	// is a re-entry guard (Task A2's forEachObject pattern): it lets a nested
	// Derived (HasKeyword inside a MatchesSpecFrom) atomically share the
	// cached list and, on the never-happens-in-practice rebuild-mid-range
	// path, build a private list instead of clobbering the outer call's.
	// Clone() copies none of these fields (see clone.go); a cloned engine
	// starts with a zero key and rebuilds identically on its first Derived.
	activeBuf     []ContinuousEffect
	activeEpoch   int
	activeVersion int
	activeDepth   int
	// continuousVersion is bumped by every direct mutation of e.continuous
	// (layers.go's AddContinuous and EndOfTurnCleanup). It stands in for the
	// events a board change would signal through the log head: while
	// AddContinuous also emits a ClockTick, EndOfTurnCleanup rewrites
	// e.continuous in place with no event, and the active() cache must see
	// that drop (an UntilEOT pump expiring) even though the log head did not
	// move. A zero value is never taken as a valid cache hit across rebuilds
	// because active() guards hits on version as well.
	continuousVersion int

	// searchingBy tracks the library search in flight, for the Opposition
	// Agent class: repl:Moved's FoundSearchingLibrary$ matches only while a
	// search's own moves are being emitted, and the search-control static
	// redirects the search pick's decision. The depth counter keeps a nested
	// search's flag alive until the outer search leaves applyLibrarySearch.
	// Synchronous engine-runtime state (set and cleared around one
	// synchronous applyLibrarySearch), never folded from an event and never
	// read across a suspension.
	searchingBy state.PlayerID
	searchDepth int

	// loop is the livelock watcher (rules/livelock.go): pure observation of
	// the event stream this engine is logging, configured by Config.
	// LoopGuard. It reads nothing and is read by nothing else; it panics
	// with a *LivelockError when the stream looks non-terminating. Clone
	// copies the guard thresholds and resets the run/quiet state (a clone
	// only happens at an intent boundary, where the watcher is idle
	// anyway), so the clone and the original watch their own streams
	// independently.
	loop livelockWatcher

	// derivedKW / derivedTypes are Derived's scratch keyword and type buffers
	// (rules/layers.go): the full Derived(struct) build rewrites them in place
	// so repeated derived-characteristic reads do not allocate. They are pure
	// per-call scratch, rebuilt from the face and active() every call, so they
	// carry no cross-call state beyond capacity; Clone() copies none of them
	// (see clone.go), so a cloned engine grows its own — never aliasing the
	// original's mutable scratch, exactly the A2 buffer / C3 digest precedent.
	// derivedDepth is the re-entry guard for the reuse (the A2/active()
	// pattern): a nested Derived mid-build owns private buffers instead of
	// clobbering the outer build's.
	derivedKW    []string
	derivedTypes []string
	derivedDepth int

	// pendingTriggers holds matched triggers not yet placed on the stack.
	// checkTriggers appends; putTriggersOnStack drains. Task 20 (trigger.go).
	pendingTriggers []pendingTrigger
	// triggerBefore is the immutable pre-departure board for an SBA death
	// batch. Scoped to its emission/resumption, never carried as live state.
	triggerBefore *triggerSnapshot
	// lifeLossBatch holds the events in one simultaneous life-loss operation.
	// It is scoped to one synchronous effect/combat pass, so it is always nil
	// at an intent boundary and does not need log encoding or Clone state.
	lifeLossBatch          []events.Event
	lifeLossBatchDepth     int
	finishingLifeLossBatch bool
	// Per-stack-instance trigger provenance, derived while queuing/placing
	// triggers, cloned at intent boundaries and removed when the stack object
	// leaves. Never encoded in events or inferred from a resolving source.
	triggerContexts map[state.ObjID]effects.TriggerContext
	// triggerLKI preserves the causing event's object snapshot from trigger
	// match through placement and resolution. TriggerPush can log Remembered
	// ids but not the pre-move object value (whose counters Move clears), so
	// this replay-derived map is the LKI analogue of triggerContexts.
	triggerLKI map[state.ObjID]triggerObjectLKI
	// sacrificedLKI maps a stack object id to the last-known-information
	// snapshot of every permanent that object sacrificed (as a cost), captured
	// at the instant of the sacrifice (Task sac1). It is engine-only, never
	// written to a state.Object, because a log-only state.Game reconstruction
	// (replayFromLog) rebuilds the stack object from the AbilityPush event
	// alone and would not reproduce an object field we set in cast.go -- the
	// same reason triggerContexts is engine-only. Resolution reads it and
	// builds effects.Ctx.Sacrificed; the entry is removed when the stack
	// object leaves, mirroring triggerContexts.
	sacrificedLKI map[state.ObjID][]state.SacrificedInfo
	// sourceLifelinkLKI maps an independently resolving ability's stack object
	// to its source permanent's derived lifelink state at the last moment that
	// source existed on the battlefield. The map's presence is the validity
	// bit: false is authoritative LKI too. It is captured before a source is
	// sacrificed as its own activation cost, refreshed for already-stacked and
	// pending abilities when their source later departs, cloned at intent
	// boundaries, and removed with the stack object. Resolution copies it into
	// effects.Ctx; effects uses it only when the source is no longer live.
	sourceLifelinkLKI map[state.ObjID]bool
	// sourceControllerLKI is the matching pre-departure controller snapshot.
	// It is separate from sourceLifelinkLKI because false lifelink is still a
	// valid snapshot, and a controller may be seat zero.
	sourceControllerLKI map[state.ObjID]state.PlayerID
	// damageSourceLKI carries snapshots keyed first by the waiting stack
	// object and then by a departed named DamageSource$ object. Unlike the
	// own-source maps above, every waiting resolution receives departures: the
	// named source can be TriggeredCard, Targeted, or Remembered.
	damageSourceLKI map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI
	// orderedTriggers is how many LEADING entries of pendingTriggers have
	// already had their order settled by an answered KTriggerOrder decision
	// (or, for a lone trigger, by there being nothing to decide). It is the
	// whole of Task 27's resumable-drain state, and it exists because the
	// queue can grow while a controller is being asked: Submit runs handle,
	// then checkStateBased, then Advance, and checkStateBased (sba.go) is a
	// fixed-point loop whose PlayerLost/MoveZone/GameOver emits each run
	// checkTriggers. Appends land at the END; decisions are always about the
	// FRONT; and while this is non-zero putTriggersOnStack does not re-sort,
	// so a trigger that arrives mid-drain can neither be shuffled into a
	// group the player has already ordered nor make them order the same
	// triggers twice. Zero whenever pendingTriggers is empty.
	orderedTriggers int
	// applyingReplacement guards re-entrancy: while a replacement effect's
	// own resolution is running, nested emits skip the replacement check
	// entirely, so a replacement that re-emits a matching event cannot
	// replace itself again. Task 20.
	applyingReplacement bool
	// replReplaced is the ev.Obj of the replacement applyReplacements is
	// currently resolving — the object the replaced event was about. It is
	// seeded by applyReplacements (Ctx.Replaced = ev.Obj) and read by Ask to
	// thread that object across a mid-resolution suspension: a ReplaceWith$
	// body that poses an ask needs Replaced (and Remembered = [that object],
	// and the X value) restored on the resume, or its Defined$ ReplacedCard
	// resolution and SVar:X Remembered$Amount gating see nothing and the
	// completed move never happens (fx44, Mox Diamond). Zero whenever no
	// replacement is in flight.
	replReplaced state.ObjID
	// replacingEvent is the in-flight Damage event a DB$ ReplaceEffect body's
	// ReplaceEvent call may rewrite (Amount/Affected). It exists only during
	// emit, before the event is logged, so it is never part of
	// cloned/replayed engine state.
	replacingEvent  *events.Event
	replacingSource state.ObjID
	// replAction is the action marker (events.ActionMarker) of the event the
	// in-flight destination-changing replacement discarded: "sacrificed",
	// "discarded" or "discarded as a cost". emit re-labels the replacement
	// body's move of replReplaced with it (events.CarryAction), so a
	// sacrifice or discard redirected by a replacement is still seen as that
	// action by Sacrificed/Discarded triggers. Empty whenever no such
	// replacement is in flight; threaded across a suspension by resumePoint.
	replAction string
	// replReplacedPlayer is the player a replaced DRAW event was about (the
	// draw-er), threaded the same way replReplaced threads the replaced
	// object: a ReplaceWith$ body over R:Event$ Draw poses mid-resolution
	// asks (Breathstealer's Crypt's unless-pay discard) and the resume must
	// restore Ctx.ReplacedPlayer. Only a Draw replacement sets it.
	replReplacedPlayer state.Target
	// triggerFireCount and the damage-batch fields below are trigger_match.go's
	// own bookkeeping (the cascade bound and the DamageDealtOnce/DamageDoneOnce
	// once-per-damage-batch gate); see there.
	triggerFireCount map[triggerKey]int32
	// A damage batch is the set of Damage events dealt simultaneously: one
	// combat-damage pass (rules/combat.go damageStep), or the Damage events
	// one dealDamage-style effect call deals (effects/damage.go brackets each
	// of those with Host.BeginDamageBatch/EndDamageBatch), or — when neither
	// brackets it — one single Damage event, opened implicitly in emit.
	// DamageDealtOnce/DamageDoneOnce latch once per batch per trigger and
	// referent (dealing source / damaged object); the entries here carry the
	// accumulated batch amount the queued trigger's referent is patched to at
	// batch close. Never opened across a drain: pendingTriggers is append-only
	// while a batch is open, so the batch entries' recorded indices stay valid.
	// damageBatchDepth counts nested brackets, so an inner effect cannot close
	// its caller's simultaneous batch early.
	damageBatchOpen  bool
	damageBatchDepth int
	damageBatchIdx   map[damageBatchKey]int
	damageBatchLog   []damageBatchEntry
	// phaseUnknownNoted memoizes the Phase$ specs whose names this engine has
	// already reported as unresolvable (rules.trigger_match.go's phaseMatches
	// reporting), so one spec emits exactly one Note per game no matter how
	// often its trigger is walked. Cloned like the other bookkeeping maps so
	// a branch that becomes live cannot re-emit the same Note.
	phaseUnknownNoted map[string]bool
	// phaseSpecs caches pure Phase$ parsing for both diagnostics and matching.
	// It is scratch, not replay bookkeeping: clones start with an empty cache.
	phaseSpecs map[string]parsedPhase
	// triggerEventMasks caches only immutable syntax for unbound fixture faces,
	// not live source membership. Bound corpus faces use their catalog-owned
	// trigger interests. Like phaseSpecs, clones own fresh writable scratch.
	triggerEventMasks map[*cards.Face]triggerEventMask
	// triggerObjectMasks is the dense object-walk form of triggerEventMasks.
	// Entries validate their immutable face pointer and are scratch owned by
	// one Engine, so hypothetical clones never share writable cache storage.
	triggerObjectMasks []objectTriggerEventMasks

	// choosing says which flow is waiting on the current KChoose decision
	// (Task 8). It is plain data, not a closure, so Engine.Clone (a sibling
	// branch, not yet in this worktree) can copy it like any other field --
	// a closure captured over this Engine's own pointers would not survive a
	// clone at all. handleChoose (turn.go) switches on it; Task 9 adds
	// chooseCast in cast.go, and Tasks 12 and 18 add the "as this enters" and
	// miracle cases in their own files.
	choosing chooseFor

	// resume is non-nil while a mid-resolution decision is pending: an effect
	// (a nested effCharm pick, effCopySpellAbility's UnlessCost$ may-pay,
	// effDiscard's mode choices — M2d-2) asked through effects.Host.Ask and
	// the resolution of the top-of-stack object is suspended with the object
	// still on the stack. It chains every suspended continuation, innermost
	// first, via resumePoint.outer (fx34): a nested ask no longer overwrites
	// its enclosing continuation, so the outer chain runs once everything
	// inside it resolves. It is plain value/pointer data (kind, obj, the
	// shared-immutable *cards.SA and the linked outer chain), never a
	// closure, so Clone copies it like cast/choosing and a replay re-derives
	// the same branch. resolveTop checks it after each resolution pass;
	// handleModes clears it and calls resumeResolution (rules/resolution.go)
	// with the recorded answer. Nil whenever no resolution is suspended.
	resume *resumePoint

	// contChain accumulates the enclosing-loop suspension points reported by
	// effects.Resolve during the current resumeResolution re-entry (through
	// effects.Host.SuspendContinuation), so resumeResolution can link them as
	// outer continuations. It is transient engine state: rebuilt on every
	// re-entry, drained into the resume chain as soon as that re-entry
	// suspends again, and nil whenever no re-entry is in flight — so a Clone
	// need not carry it (the same resolution re-derives the same chain).
	contChain []contFrame
	// resolvingObj is the stack object whose resolution is running (resolveTop
	// or a resumed resolution), kept through its final move off the stack so
	// an entry replacement that asks can tell whether it interrupted that
	// resolution's own move. Zero outside a resolution.
	resolvingObj state.ObjID
	// repeatReported is the RepeatEach SA whose loop frame SuspendRepeat
	// just recorded, so the enclosing Resolve loop's report of the same SA
	// is not recorded a second time as a plain continuation.
	repeatReported *cards.SA

	// cast holds the in-progress cast-flow state while choosing ==
	// chooseCast (Task 9, rules/cast.go). Nil whenever no cast is mid-flow.
	cast *pendingCast
	// riotMove parks a non-cast battlefield entry while its controller makes
	// Riot's as-enters choice. The event is emitted only after Choose records
	// the answer, so every entry path reaches events.Move with RiotChoice set.
	riotMove *events.Event
	// suspendedCasts is the mandatory "cast it if able" trigger created when
	// a real suspended card loses its final TIME counter. IDs are appended in
	// exile order and consumed before priority; it is plain replayable engine
	// continuation state, not an inference from arbitrary exile cards.
	suspendedCasts []state.ObjID
	// manaActivation is non-nil while a source with several available mana
	// abilities waits for its controller to select one. manaColorActivation
	// similarly holds an already-paid Produced$ Any ability, and
	// manaDiscardActivation holds an ability whose discard cost is being
	// chosen. All are plain data so Clone preserves an offered activation.
	manaActivation        *manaActivation
	manaColorActivation   *manaColorActivation
	manaDiscardActivation *manaDiscardActivation
	manaUnlessActivation  *manaUnlessActivation
	// unlessPayment carries an in-progress non-mana unless-cost payment. It
	// keeps the enclosing resolution suspended while the payer chooses the
	// sacrifice/discard objects that pay it.
	unlessPayment *unlessPayment
	// Resolution-time payment windows. cumulative belongs to the replayable
	// keyword trigger; triggerCost belongs to an ordinary triggered effect
	// carrying Cost$ (Mana Vault). Both are plain data and Clone-copied.
	cumulative  *cumulativeUpkeep
	triggerCost *triggeredEffectCost
	// echo (rules/echo.go, kw:Echo): the pay-or-sacrifice election of a
	// resolving echo keyword trigger. Same plain-data class as the two
	// above; Clone-copied.
	echo *echoFlow

	// wardMana holds a CR 702.21a mana-payment window while a Ward trigger
	// is resolving. It is plain data so Clone preserves the suspended choice.
	wardMana *wardManaPayment

	// cmdZone is the queue of parked commander zone changes (CR 903.9, Task
	// m32, rules/replacement.go): MoveZone events a commander is about to
	// undergo, deferred until its owner answers the KCommanderZone decision
	// for the FRONT entry. Multiple commanders can be parked by one burst (a
	// board wipe, one state-based-action pass), but only one decision can be
	// pending at a time, so the queue hands from one answer to the next
	// (handleCmdZone asks the new front after emitting the old one). Entries
	// are deduplicated by object id, so a re-offered SBA pass can never
	// enqueue the same commander twice (the no-progress failure shape a
	// replacement must not spin on). Never mutated while e.pending is nil
	// except by a park that also asks; Clone deep-copies it (clone.go) so a
	// clone taken with a decision outstanding carries the same queue. It is
	// always empty in a non-Commander game: nothing ever parks there.
	cmdZone []cmdZoneMove
	// replChoices is the queue of parked replacement choices (see replChoice /
	// handleReplacement in replacement.go): CR 616.1 ordering for MoveZone,
	// Untap, ProduceMana and BeginPhase, replacement-time mana-colour choices,
	// and an Optional$ BeginPhase yes/no. Plain value entries are deep-copied by
	// Clone, so every in-flight event survives an intent boundary.
	replChoices []replChoice
	// untapResume is set only around one Untap emission from finishUntapStep.
	// If that event parks an Untap replacement choice, it moves into the queue.
	untapResume *untapStep
	// madnessChoices parks discard moves while the card's owner decides whether
	// to apply Madness's optional hand-to-exile replacement.
	madnessChoices []events.Event
	// applyingMadnessChoice suppresses only the Madness interposition while an
	// answered choice emits its selected destination.
	applyingMadnessChoice bool

	// suppressedCast holds the card object ids whose cast option is held out
	// of the current priority window because their cast attempt aborted
	// unpayable with no state change (E2 round 2, tightened by F05-2/CR
	// 733.2). This is the no-progress answer for a hash-chained, replayable
	// engine: instead of counting no-progress aborts and killing the match,
	// suppress the exact card that produced one, so the re-offer loop cannot
	// begin an unbounded second iteration -- nobody's match dies, and the
	// seat may still do anything else. F05-2 (CR 733.2) lets a reversed
	// illegal action be redone legally, so suppression engages only on the
	// SECOND identical no-progress abort of the same card in the same window
	// (the per-card castAborts count), never the first. Cleared on any
	// genuinely state-changing event (see emit), so a declined card's option
	// comes back the moment the window ends or the mana/board changes. The
	// id already names the one seat that holds it, so two different cards'
	// declines never interact and two seats' never do either.
	suppressedCast map[state.ObjID]bool

	// castAborts counts the no-progress cast/activation aborts per card so far
	// in the current priority window (F05-2, CR 733.2): the held-out
	// suppression above engages only on the SECOND identical no-progress abort
	// of the same card, so a merely-reversed illegal action (CR 733.2) may be
	// redone legally once -- the first abort leaves the option offered -- while
	// a deliberate repeat still cannot spin the engine. It is cleared by the
	// same state-changing-event rule that clears suppressedCast, so the count
	// and the held-out set have identical lifetimes. Like suppressedCast it is
	// transient window bookkeeping on the Engine, never an event or a
	// state.Game field, so a replay re-derives it by re-running the same
	// aborts rather than reading it from the log.
	castAborts map[state.ObjID]int32

	// drainAwaitsTarget is true while a decision asked from inside the trigger
	// drain is pending, so its answer resumes the drain rather than granting
	// priority. Task 7 sets it for a TargetMin/TargetMax-bearing triggered
	// ability's own KTarget decision (pushTrigger, cleared by handleTarget);
	// Task 18 sets it for a Miracle cast's own X/Delve/Sac decision
	// (castMiracle, cleared by handleChoose once the cast commits). When set,
	// the handler for the pending decision calls resumeTriggerDrain (the same
	// continuation handleTriggerOrder uses) instead of granting the caster
	// priority, so a later, unrelated trigger in the same batch is still
	// placed before any player acts. Plain scalar, so Clone copies it like
	// every other field here. (Tasks 7, 18.)
	drainAwaitsTarget bool

	// drainAwaitsModes is the CR 603.3c twin of drainAwaitsTarget: true while
	// a modal triggered ability's KModes decision asked at placement
	// (pushTrigger) is pending, so its answer records the chosen modes onto
	// the stack object and resumes the drain (handleModes) rather than
	// granting priority. Plain scalar, Clone copies it, and a replay re-derives
	// the same branch from the same recorded answer.
	drainAwaitsModes bool

	// deferCastTrigger is set only around the up-front cast push (CR 601.2a)
	// emit in pushCast. While it is true, emit HOLDS the PutOnStack event's
	// cast trigger back instead of running checkTriggers for it, because the
	// spell is not yet actually cast: the "when you cast" trigger (601.2i)
	// fires only after the target choice (601.2c) and payment (601.2h). The
	// held event is stored in deferredPush and re-checked by payCast's
	// fireDeferredCastTrigger. A bool (not a count) is safe because emit is
	// single-threaded and the push's Apply/transformation path never re-enters
	// emit for another PutOnStack; a replacement that fires here (as for any
	// PutOnStack) recurses on the OTHER kind, which falls to checkTriggers
	// normally. Zero whenever no cast push is in flight, so Clone copies it.
	deferCastTrigger bool

	// deferredPush holds the up-front PutOnStack event of an in-flight cast
	// whose cast trigger (CR 601.2i) is held back until the cast is complete
	// (see deferCastTrigger). Zero when nothing is deferred. payCast's
	// fireDeferredCastTrigger re-walks it once the spell is paid for; an
	// aborted proposal drops it. Each is a pointer so a Clone taken with a
	// cast in flight copies the held event (a replay re-derives the same
	// trigger from the recorded PutOnStack).
	deferredPush *events.Event

	// deferredPushLKI is the LKI snapshot captured for deferredPush's own
	// Obj when pushCast emitted it, threaded into the trigger walk so a
	// ChangesZone trigger fired by the cast (see spellCastMatches) can read
	// the card as it was just before the stack move.
	deferredPushLKI *state.Object

	// noCounterSpend is the transient capture of emitRestrictedManaSpend: the
	// id of the SPELL whose payment just consumed a batch carrying
	// AddsNoCounter$ provenance (Cavern of Souls' "that spell can't be
	// countered"), zero when none. payManaCast's caller (payCast) reads it
	// once, synchronously, right after the payment — no ask can suspend
	// between the spend and the read (emitRestrictedManaSpend emits, never
	// asks) — and folds state.FlagNoCounter into the pay-time CastInfo, so
	// replay re-derives the flag from the recorded event exactly like every
	// other cast flag. Zero whenever no such spend is in flight, so Clone
	// copies nothing of it.
	noCounterSpend state.ObjID

	// costProvenanceSeen is the transient capture of the last cost-modifier
	// pass (castprov3): true when that pass evaluated a cost static whose
	// ValidCard$ carries a cast-provenance token (Bilbo's
	// "!wasCastFromYourHand" ReduceCost) — such a static is unresolvable
	// pre-push, so the pass denied it and the pending cast's payment needs
	// the post-push re-price continueCast runs right after CR 601.2a's push.
	// Set inside costStaticApplies (inside the costModifiers attribution
	// roots, so the param census sees no new read), cleared at the top of
	// every costModifiersWithTargets[ X]Using pass. Like noCounterSpend it
	// is synchronous computation state: every read of it (the option-
	// selection sites and continueCast's post-push re-price) happens in the
	// same driven flow as the pass that set it, and no ask suspends between
	// the pass and the read. Like noCounterSpend, Clone copies nothing of
	// it.
	costProvenanceSeen bool

	// damaging names the source object responsible for the damage emit
	// currently in flight (CR 609.7a): the resolution source for a spell or
	// ability being resolved, or the dealing creature for a combat
	// assignment. Task 15 sets it around resolveTop's two resolution
	// calls and combat's assignment loop, and emit consults it to prevent
	// damage to a protection-bearer whose protecting quality the source
	// carries (CR 702.16d). Zero when nothing is resolving/assigning damage;
	// a zero damaging never suppresses a Damage event.
	//
	// Not copied by Clone (Task 15 fix round 1, M5): Clone runs only at an
	// intent boundary, after New/Advance/Submit has returned, at which point
	// every resolution and damage-step that EVER sets damaging has completed
	// and reset it to zero -- an unset non-zero damaging would mean an emit
	// was still in flight, which is exactly the boundary Clone is prohibited
	// from crossing. So the field is always zero at a clone boundary and
	// copying it would copy a constant.
	damaging state.ObjID

	// combatDamaging distinguishes a combat-damage Damage event from a
	// noncombat one at trigger-match time (CombatDamage$ True/False, CR
	// 702.1x names the combat damage step's own assignments) WITHOUT touching
	// events.Event: the event struct's binary encoding is hash-chained and
	// replayed, so per-event context lives in engine state and is rebuilt by
	// replay because replay re-executes the same setter (the pg2 precedent).
	// dealCombatDamage (combat.go) sets it true alongside e.damaging for the
	// length of each assignment and clears it with the same reset; triggers
	// are checked synchronously inside emit (checkTriggers on the stored
	// event), so the flag is valid at match time. Every non-combat Damage
	// site -- the resolving ability in resolveTop/resumeResolution and the
	// effects/damage.go primitives those wrap -- leaves it false, and a
	// DealDamage cast during the combat damage step is still NOT combat
	// damage. Not copied by Clone, for the same reason as damaging above:
	// always zero at a clone boundary.
	combatDamaging bool

	// manaFromTap and manaProducer identify the mana ability currently
	// resolving. They are synchronous context rather than ManaAdd fields.
	manaFromTap  bool
	manaProducer state.ObjID
	// stepLeaving is the step transition currently offered to BeginPhase
	// replacements; parked choices own a value copy.
	stepLeaving *state.Step

	// Tapping and damage provenance are likewise synchronous event context.
	tappingForMana      state.ObjID
	tappingManaProduced string
	tapObj              state.ObjID
	tapPlayer           state.PlayerID
	tapEntering         bool
	tappedTurn          map[state.ObjID]int32
	triggerTurnFires    map[triggerKey]turnFires
	// triggerTurnResolved is ResolvedLimit$'s per-turn resolution count,
	// keyed by the trigger's SOURCE object (not its triggerKey): Forge's
	// TriggeredAbility.resolvedThisTurn caps how many times a T: line may
	// RESOLVE each turn, and a ResolvedLimit$ card's paired lines (the
	// corruption_of_towashi halves of one printed ability) must share it.
	triggerTurnResolved map[state.ObjID]turnFires
	dmgSrcOverride      state.ObjID
	batchLifelink       map[state.ObjID]bool

	// foreachBuf is forEachObject's (trigger_match.go) scratch snapshot
	// buffer. forEachObject copies each zone into it before walking it -- fn
	// may move objects between zones (a trigger match putting something on
	// the stack), so iterating the live, mutating zone slice would be a bug.
	// append(buf[:0], zone...) grows it in place, so it settles at the size
	// of the largest zone seen and then stops allocating -- a fresh zone
	// copy per zone per event used to be forEachObject's 11.51 GB allocation
	// footprint (Task A2), the single largest allocator in this package.
	// Owned by this Engine alone: Clone leaves both fields zero, so a clone
	// and the original share no snapshot mid-walk (the clone just lets it
	// grow again). foreachDepth guards re-entry (see forEachObject): zero
	// outside a walk, one inside the depth-0 walk, higher inside a
	// re-entrant nested walk.
	foreachBuf   []state.ObjID
	foreachDepth int
}

// inFlightDamageSource is the one reader for Damage-event provenance: the
// published override when a damage emitter set one, else the resolution/combat
// source e.damaging carries. Zero when neither is set (a Damage event with no
// recorded source -- emit's protection check treats zero as "never prevent",
// and damageMatches fails the ValidSource$ match).
func (e *Engine) inFlightDamageSource() state.ObjID {
	if e.dmgSrcOverride != 0 {
		return e.dmgSrcOverride
	}
	return e.damaging
}

// SetDamageSource implements effects.Host: publish the damage source for the
// Damage events the calling emitter is about to emit, returning the previous
// value so the emitter restores it. See dmgSrcOverride's field doc for the
// replay/clone discipline.
func (e *Engine) SetDamageSource(id state.ObjID) state.ObjID {
	prev := e.dmgSrcOverride
	e.dmgSrcOverride = id
	return prev
}

// BatchDepartures implements effects.Host: snapshot the derived lifelink
// state of every object the caller is about to move in one destruction
// batch, so each member's departure capture reads the pre-batch state no
// matter where it sits in battlefield order. See batchLifelink's field doc
// for the consumption discipline.
func (e *Engine) BatchDepartures(ids []state.ObjID) {
	e.batchLifelink = make(map[state.ObjID]bool, len(ids))
	for _, id := range ids {
		e.batchLifelink[id] = e.HasKeyword(id, "Lifelink")
	}
}

// EndBatchDepartures closes a destruction/sacrifice batch even if one of its
// proposed moves was prevented or replaced. Without this explicit boundary,
// that survivor's pre-batch LKI could be consumed by an unrelated later move.
func (e *Engine) EndBatchDepartures() { e.batchLifelink = nil }

// chooseFor names the flow a pending KChoose decision belongs to. Task 9
// declares chooseCast (rules/cast.go); Tasks 12 and 18 add the "as this
// enters" and miracle cases in their own files.
type chooseFor uint8

const chooseNone chooseFor = iota

// commanderCardLegal reports whether ONE card may be a commander under
// CR 903.3. It DELEGATES to deck.IsCommanderEligible -- the one
// implementation the deck validator (File.ValidateCommander) also uses -- so
// the validator and the engine can never disagree about which cards seat:
// a legendary creature, a legendary Vehicle, a legendary Spacecraft with a
// printed power/toughness box (Hearthhull, the Worldseed), or a card whose
// printed/Oracle text says it can be your commander (the planeswalker
// commanders and the Partner/choose-a-background cases). The old inline
// predicate here (a legendary creature on Faces[0], commander permission
// read from Keywords only) was the stale half of a two-implementation
// disagreement and rejected decks the validator accepted.
func commanderCardLegal(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	return deck.IsCommanderEligible(c)
}

// partnerHead reports the Partner-family head c carries, "" for none: the
// plain Partner ability (whose "Friends forever" alias spells K:Partner:...
// and shares the head, CR 903.13a), or "Partner with" (the CR 903.13c named
// pair).
func partnerHead(c *cards.Card) string {
	if c == nil || len(c.Faces) == 0 {
		return ""
	}
	for _, k := range c.Faces[0].Keywords {
		h := cards.KeywordHead(k)
		if h == "Partner" || h == "Partner with" {
			return h
		}
	}
	return ""
}

// partnerPairOK reports whether two cards may be a commander PAIR: each
// carries a Partner-family ability and either both are plain Partners, or
// each "Partner with" the other by printed name (CR 903.13a/c). A plain
// Partner paired with a Partner-with card is not a legal pair (each half of
// a named pair names its own partner); a Partner-with card paired with a
// plain Partner fails the same way.
func partnerPairOK(a, b *cards.Card) bool {
	ha, hb := partnerHead(a), partnerHead(b)
	if ha == "" || hb == "" {
		return false
	}
	if ha == "Partner" && hb == "Partner" {
		return true
	}
	return partnerWithNames(a, b.Faces[0].Name) && partnerWithNames(b, a.Faces[0].Name)
}

// partnerWithNames reports whether c carries a "Partner with" whose named
// partner is other (the corpus form is "Partner with:<name>[:<display>]";
// the first colon-field is the name).
func partnerWithNames(c *cards.Card, other string) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	for _, k := range c.Faces[0].Keywords {
		if cards.KeywordHead(k) != "Partner with" {
			continue
		}
		_, rest, ok := strings.Cut(k, ":")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, ":")
		if strings.EqualFold(strings.TrimSpace(name), other) {
			return true
		}
	}
	return false
}

// legalCommandersFor validates seat i's configured commander list against
// the deck-construction rules (CR 903.4/903.13) and returns the indices
// that MAY be seated, in Config order: a single commander must be a
// legendary creature or a "can be your commander" card; a two-card seat is
// a legal partner pair (plain Partners, or a mutual "Partner with" pair);
// anything else -- a noncommander card, a pair without partner, more than
// two -- is rejected WHOLE, never silently trimmed into a legal-looking
// subset. This is what makes an illegal Config fail in play: the rejected
// seat plays commander-less and the rejection is on the log as a Note (the
// same degrade-don't-crash stance New takes for malformed decks elsewhere --
// New cannot return an error, so the Note is the record).
func (c *Config) legalCommandersFor(i, deckLen int, deck []*cards.Card) []int {
	raw := c.commandersFor(i, deckLen)
	if len(raw) == 0 {
		return nil
	}
	// Construction legality belongs to Commander games. Config.Commanders also
	// intentionally powers constructed-format fixture and compatibility paths
	// (where it merely selects command-zone objects), so preserve that legacy
	// plumbing outside FormatCommander.
	if c.Format != FormatCommander {
		return raw
	}
	bad := func(why string) ([]int, string) {
		names := ""
		for _, idx := range raw {
			if idx >= 0 && idx < len(deck) && deck[idx] != nil {
				names += deck[idx].Faces[0].Name + ", "
			}
		}
		return nil, names + why
	}
	reject := ""
	switch len(raw) {
	case 1:
		idx := raw[0]
		if idx >= 0 && idx < len(deck) && !commanderCardLegal(deck[idx]) {
			_, reject = bad("is not a legendary creature and does not say it can be your commander")
		}
	case 2:
		a, b := raw[0], raw[1]
		inRange := func(x int) bool { return x >= 0 && x < len(deck) && deck[x] != nil }
		if !inRange(a) || !inRange(b) {
			_, reject = bad("is not a card this deck carries")
		} else if !commanderCardLegal(deck[a]) || !commanderCardLegal(deck[b]) || !partnerPairOK(deck[a], deck[b]) {
			_, reject = bad("is not a partner pair")
		}
	default:
		_, reject = bad("is not one or two commanders")
	}
	if reject != "" {
		return nil
	}
	return raw
}

// commandersFor returns the VALID commander indices (into deck of length
// deckLen) that Config names for seat i, in Config order. An index out of
// range for the deck, or a seat with no Commanders entry, contributes
// nothing -- the same degrade-don't-crash stance New already takes for more
// decks than seats: the bogus entry is skipped, never a panic.
func (c *Config) commandersFor(i, deckLen int) []int {
	if i < 0 || i >= len(c.Commanders) {
		return nil
	}
	var out []int
	for _, idx := range c.Commanders[i] {
		if idx >= 0 && idx < deckLen {
			out = append(out, idx)
		}
	}
	return out
}

const openingHand = 7

func New(cfg Config) *Engine {
	return newWithRNG(cfg, newRNG(cfg.Seed))
}

func newWithRNG(cfg Config, random *rng) *Engine {
	life := int32(20)
	if cfg.StartingLife > 0 {
		life = cfg.StartingLife
	}
	initialObjects := 0
	for i, deck := range cfg.Decks {
		if i >= len(cfg.Names) {
			break
		}
		initialObjects += len(deck)
	}
	e := &Engine{
		G:            state.NewGameLife(cfg.Names, life, initialObjects),
		L:            events.NewLog(cfg.Seed),
		format:       cfg.Format,
		rng:          random,
		loop:         newLivelockWatcher(cfg.LoopGuard),
		turnsTaken:   make([]int32, len(cfg.Names)),
		compiledText: newCompiledText(cfg),
	}
	e.G.Tokens = cfg.Tokens
	e.format = cfg.Format
	for i := range e.G.Players {
		if i < len(cfg.PlayerNames) && cfg.PlayerNames[i] != "" {
			e.G.Players[i].PlayerName = cfg.PlayerNames[i]
		}
	}
	e.emit(events.Event{Kind: events.GameStart, Amount: int32(len(cfg.Names))})
	// CR 103.1: the starting player is determined by a random method. Draw
	// the toss HERE, as the FIRST rng consumption of the game, before any
	// per-seat shuffle: the toss value is then a pure function of (seed,
	// seat count), independent of every deck size. The surviving-seat
	// resolution happens after the deal below (the toss draw is uniform over
	// every seat, so conditioned on naming a survivor it is uniform over the
	// survivors -- see the resolution site). CR 103.1's second half -- the
	// toss winner CHOOSES who takes the first turn -- is not implemented;
	// see the "Known approximations" row in AGENTS.md.
	toss := -1
	if len(cfg.Names) > 0 {
		toss = e.rng.IntN(len(cfg.Names))
	}
	// CR 103.1 precedes 103.2-103.4: the public toss announcement is emitted
	// HERE -- before the first shuffle and the opening hand (Forge's
	// GameAction and manabrew's game loop announce the toss before their deal
	// too), so the keep/mulligan decisions are made with the toss already
	// public. The Note names the seat the rng handed the toss to -- the true
	// CR 103.1 winner -- even if the deal below then eliminates them; who
	// actually takes the first turn is resolved after the deal has fixed the
	// survivors. The text carries the deck identity, never the display
	// PlayerName (F3 keeps display names out of the chain); view/describe.go
	// renders this one Note through player(), so a seated human still reads
	// their own name.
	if toss >= 0 {
		e.emit(events.Event{Kind: events.Note, Player: state.PlayerID(toss),
			Text: tossName(e.G, state.PlayerID(toss)) + " won the toss"})
	}
	// Match-wide dense commander indexing for Player.CmdDamage (assigned at
	// genesis): a commander's dense index is the sum of (valid commanders in
	// seats before its owner) + (its own position within its owner's
	// Commanders list, which is the order Commanders is built in the loop
	// below) -- both deterministically derivable from this Config, so no
	// separate index needs storing. Every seat's CmdDamage is sized to total
	// (the whole match's commander count) so it can be indexed by ANY
	// commander's match-wide dense index: seat B's damage holds a slot for
	// seat A's commander at A's commander's dense index. Sized here and
	// never grown; three small copy() calls per seat carry all three across
	// Game.Clone.
	totalCmd := 0
	for i := range cfg.Names {
		if i < len(cfg.Decks) {
			totalCmd += len(cfg.legalCommandersFor(i, len(cfg.Decks[i]), cfg.Decks[i]))
		}
	}
	// Opening hands are dealt as one genesis operation. Defer only the final
	// GameOver event: drawCard still emits losses and runs all other SBAs, but
	// the terminal marker must follow the public toss Note so the host can
	// recognize and persist the complete genesis burst.
	e.deferGameOver = true
	for i, deck := range cfg.Decks {
		if i >= len(cfg.Names) {
			// Ruling T22-m (fix round 2): a malformed Config with more
			// decks than named seats has nowhere to put the rest --
			// state.NewGame above sizes g.zones from len(cfg.Names) alone,
			// so PlayerID(i) here would index outside it and panic
			// (SetZone -> zoneIndex -> an out-of-range g.zones write).
			// Task 25 wires Config from a client, so a malformed one must
			// degrade, not crash the one goroutine running the whole
			// match; the excess decks are simply never dealt, the same
			// spirit as the zero-alive guard below for a Config with no
			// seats at all.
			break
		}
		p := state.PlayerID(i)
		ids := make([]state.ObjID, 0, len(deck))
		for _, c := range deck {
			ids = append(ids, e.G.AddObject(c, p).ID)
		}
		e.G.SetZone(state.ZLibrary, p, ids)
		// Commanders leave the library for the command zone here, BEFORE the
		// shuffle and BEFORE the opening hand is dealt, so they are neither
		// shuffled into the library nor drawable. Emitted as real MoveZone
		// events (one per commander, in Config order) -- the log is the only
		// source of truth, and replay, which folds the logged events back
		// through this same New, reproduces the identical command zone. For a
		// non-Commander Config commandersFor is empty, so nothing is emitted
		// and the Shuffle below covers the whole library exactly as before.
		var myCmds []state.ObjID
		for _, idx := range cfg.legalCommandersFor(i, len(deck), deck) {
			id := ids[idx]
			myCmds = append(myCmds, id)
			e.emit(events.Event{Kind: events.MoveZone, Obj: id,
				From: state.ZLibrary, To: state.ZCommand})
		}
		// A commander list that failed the deck-construction validation seats
		// nothing; the rejection is on the log (rules/legal... engine.go's
		// legalCommandersFor) so a transcript shows why the command zone is
		// empty.
		if len(myCmds) == 0 && len(cfg.commandersFor(i, len(deck))) > 0 {
			e.emit(events.Event{Kind: events.Note, Player: p,
				Text: "commander configuration rejected under CR 903.4/903.13"})
		}
		e.G.Players[p].Commanders = myCmds
		if len(myCmds) > 0 {
			e.G.Players[p].CmdCasts = make([]int32, len(myCmds))
		}
		if totalCmd > 0 {
			e.G.Players[p].CmdDamage = make([]int32, totalCmd)
		}
		// Shuffle only what is left in the library -- the commanders have just
		// moved out, so a non-Commander seat's library and the original ids
		// are one and the same and the event is byte-identical to before.
		order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, p)...)
		order = e.ShuffleLibrary(p, order)
		// Library order is hidden information: the event carries it because the
		// server needs it, and view projection redacts it for everyone else.
		e.emit(events.Event{Kind: events.Shuffle, Player: p, IDs: order, Secret: true})
		for j := 0; j < openingHand; j++ {
			e.drawCard(p)
		}
		if e.G.AliveCount() <= 1 {
			// Preserve T22-c's terminal-deal boundary: once at most one seat
			// remains, do not shuffle or deal a later hand. A later seat whose
			// configured library could not supply seven cards is nevertheless
			// also doomed by this same opening deal; account for that loss so
			// an all-undersized table truthfully reaches CR 104.4a's no-survivor
			// draw rather than accidentally crowning an undealt short deck.
			for next := i + 1; next < len(cfg.Decks) && next < len(cfg.Names); next++ {
				available := len(cfg.Decks[next]) - len(cfg.commandersFor(next, len(cfg.Decks[next])))
				if available < openingHand && !e.G.Players[next].Lost {
					e.emit(events.Event{Kind: events.PlayerLost, Player: state.PlayerID(next), Text: "drew from an empty library"})
				}
			}
			if e.finishTerminalGenesis() {
				return e
			}
		}
	}
	if e.finishTerminalGenesis() {
		return e
	}
	e.deferGameOver = false
	alive := e.G.AliveFrom(0)
	// CR 103.1: the starting seat is the toss result, not seat 0.
	// Resolve the starting seat uniformly over the SURVIVORS. The pre-shuffle
	// toss draw is uniform over every seat, so CONDITIONED on naming a
	// survivor it is already uniform over the survivors -- it is the first
	// candidate and costs no further rng. Only when it named a seat the deal
	// eliminated does rejection sampling draw again: IntN over all seats,
	// retried until a survivor is hit, each round uniform over the survivors
	// once conditioned. In the no-elimination case -- every real game -- the
	// stream is exactly one IntN and the candidate is always the first. A
	// modulo over the survivor count instead would be BIASED: three seats
	// with seat 0 eliminated maps two of the three toss outcomes onto one
	// survivor (measured 395/205 over 600 seeds on the pre-fix code).
	start, _ := e.resolveToss(toss, alive, len(cfg.Names))
	// The resolved toss is authoritative genesis state, not merely a Note or
	// the later TurnChange: opening-hand effects and Count$StartingPlayer run
	// before turn one. Fold it through events.Apply without appending a new
	// event: genesis is replayed from Config (including its seeded toss), and
	// preserving the historic event stream keeps recorded matches replayable.
	events.Apply(e.G, events.Event{Kind: events.StartingPlayerChange, Player: start})
	// The starting seat is the toss winner resolved over the survivors --
	// never seat 0 (the pre-toss assumption Ruling T22-f removed) and never a
	// seat the deal eliminated: an early seat that decked out during its own
	// opening draw (Over still false, since other seats remain, but that
	// seat's own Lost is true) must not receive turn 1. A player already out
	// of the game is simply skipped in turn order everywhere else (NextAlive,
	// priority); resolveToss is genesis's own equivalent for the very first
	// turn.
	if !e.G.Over {
		// CR 103.1's resolution, now that the deal has fixed the survivors:
		// beginTurn records start in its ordinary TurnChange. The resolved seat
		// is also state.Game.StartingPlayer now (folded above without a new
		// event: genesis is replayed from Config, including its seeded toss, so
		// preserving the historic event stream keeps recorded matches
		// replayable), which is what view's pregame projection and the
		// Count$StartingPlayer head read.
		if cfg.Mulligans > 0 {
			// Ruling R-8.4: the London mulligan round lives between the deal
			// and turn 1. e.pregame makes step() dispatch to stepPregame
			// (rules/mulligan.go) instead of the ordinary turn steps; the
			// round's end calls beginTurn below. Over is already false (the
			// per-seat deck-out guard above returned early) -- a game that
			// ended during the deal never starts a round.
			// CR 103.5: the starting player declares first, then each other
			// player in turn order -- AliveFrom(e.G.StartingPlayer) is that
			// order, which is also beginTurn's seat at the round's end. The
			// opening-hand effects round runs after this round (a Gemstone
			// Caverns may not be used from a hand its owner later mulliganed
			// away), and an accepted Impatient Iguana there replaces the
			// recorded designation before turn one.
			e.pregame = true
			e.mulligan = newMulliganRound(e.G.AliveFrom(e.G.StartingPlayer), cfg.Mulligans)
		} else {
			e.opening = e.newOpeningRound(e.G.StartingPlayer, 0)
			if len(e.opening.effects) > 0 {
				e.stepOpening()
				return e
			}
			e.beginTurn(e.G.StartingPlayer)
		}
	}
	return e
}

// finishTerminalGenesis finalizes a game whose opening deal left at most one
// survivor. The toss was already announced before the first shuffle (the
// pre-deal Note in New); terminal genesis therefore needs no additional Note.
// Ruling T22-e: nobody survived genesis is CR 104.4a's draw; one survivor is
// CR 104.2a's winner. GameOver remains the final genesis event for the host's
// persistence/replay burst boundaries.
func (e *Engine) finishTerminalGenesis() bool {
	if e.G.AliveCount() > 1 {
		return false
	}
	e.deferGameOver = false
	e.checkGameOver()
	return true
}

// resolveToss maps the pre-shuffle random determination onto the seats that
// survived the opening deal. The original candidate is already uniform over
// every configured seat; rejection sampling an eliminated candidate preserves
// uniformity over survivors without consuming another draw in ordinary games.
func (e *Engine) resolveToss(toss int, alive []state.PlayerID, seats int) (state.PlayerID, bool) {
	if toss < 0 || len(alive) == 0 || seats <= 0 {
		return 0, false
	}
	candidate := state.PlayerID(toss)
	for {
		for _, p := range alive {
			if p == candidate {
				return candidate, true
			}
		}
		candidate = state.PlayerID(e.rng.IntN(seats))
	}
}

// tossName is the identity the toss Note's text carries: the deck-identity
// Name, else "seat N". F3 (TestPlayerNamesDoNotReachTheChain) keeps the
// per-seat PlayerName -- a display name -- out of the event chain entirely,
// and the Note is event text, so it uses the same deck identity every other
// event text already carries. (view/describe.go's player label may prefer
// PlayerName; that is a view projection, not chain text.)
func tossName(g *state.Game, p state.PlayerID) string {
	pl := g.Players[p]
	if pl.Name != "" {
		return pl.Name
	}
	return "seat " + strconv.Itoa(int(p))
}

// emit is the engine's single mutation entry point. Task 20 inserts
// replacement effects ahead of logging and trigger discovery behind it:
// applyReplacements runs first (skipped, via applyingReplacement, while
// another replacement's own resolution is already in flight, which is what
// keeps a self-replacing event from looping), and if it substitutes the
// event, the original is discarded rather than logged -- the substitute's
// own emit (from inside effects.Resolve) already logged whatever needed
// logging. Otherwise the event is logged and folded into state exactly as
// before, and checkTriggers then looks for anything it just made true.
func (e *Engine) emit(ev events.Event) events.Event {
	// Task 15 protection (CR 702.16d/e): a Damage event dealt to a
	// protection-bearer by a source it is protected from is prevented -- the
	// damage never happens, reported as a Note rather than silently dropped.
	// Same for an Attach whose target is protected from the attachment -- CR
	// 702.16e: attachment points defer to protection before an Equip/Enchant
	// resolves onto a protected permanent. Both are checked BEFORE
	// replacement substitution: prevention is unconditional and must not be
	// handed to card text as if it had actually happened (and the recursive
	// emit for the Note re-enters cleanly because a Note matches neither
	// clause). The damage source is the published override when a
	// DamageSource$ emitter set one, else e.damaging; a zero source (no
	// source recorded) never suppresses a Damage event. A planeswalker's
	// Damage event is protected exactly like any other now -- its CR 306.8
	// loyalty conversion happens one fold later, in events.Apply, so a
	// prevented hit converts nothing. stat:CantPreventDamage (Spider-Punk)
	// overrides protection's own damage-prevention arm exactly like every
	// other prevention path, so the same cantPreventDamage gate applies here.
	if ev.Kind == events.Damage && ev.Obj != 0 {
		if src := e.inFlightDamageSource(); src != 0 && e.protectedFrom(ev.Obj, src) &&
			!e.cantPreventDamage(src, ev.Obj) {
			// Amount rides the stored Note (task dponce1): a prevention is a
			// game action a triggered ability can see, and Mode$
			// DamagePreventedOnce keys on these Notes' Amount.
			return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Player: ev.Player,
				Amount: ev.Amount, Text: "prevented: protection"})
		}
	}
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 &&
		e.protectedFrom(ev.IDs[0], ev.Obj) {
		// Task 15 fix round 1 (Important I1): this clause is defence-in-depth.
		// Under the five registered colour protections there is NO reachable
		// path that fires it -- askTarget withholds a coloured Aura from ever
		// targeting a protected permanent (CR 702.16c) so no such Attach is
		// ever offered to resolve, and for Equip the legalTargets fizzle in
		// resolveTop fires first (the equipment's source is colourless, so a
		// colour-protected permanent was never going to be non-legal by
		// protection anyway). The clause stays because it is the engine's one
		// guard for the moment a future task registers type or "everything"
		// protection (or an Aura that enters the battlefield pre-attached, or
		// any Attach emit produced without a targeting step) makes a actually
		// protected permanent the direct object of an Attach; removing it now
		// would silently re-open that hole. Do not test it by driving emit
		// directly -- that would prove only that the if-lookup works, not that
		// a game state reaches it.
		return e.emit(events.Event{Kind: events.Note, Obj: ev.Obj, Text: "cannot attach: protected"})
	}
	// Role-token exclusivity (the second sentence of every Role token's rules
	// text: "If you control another Role on it, put that one into the
	// graveyard."): a second Role token attaching to a bearer that already
	// carries one puts the old Role into its owner's graveyard BEFORE the new
	// Attach applies. The measured corpus makes the sweep unconditional: every
	// one of the 41 `TokenScript$ role_` carrier lines mints the Role with
	// `TokenOwner$ You` (or omits it, defaulting to the controller), so creator
	// and Role controller are always the same player and the controller-qualified
	// reading is vacuous. This lives here in emit -- beside the protection guard
	// above, the exact same pre-apply, re-entrant Attach interception -- because
	// EVERY attach path (effects/token.go's AttachedTo$ mint and
	// effects/attach.go's Attach SA) funnels through Engine.Emit. The sweep is a
	// plain MoveZone (NOT destruction: no replacement/regeneration path), whose
	// recursive emit lets "leaves the battlefield" triggers on the old Role fire
	// normally; zone slices are copied before iteration because the MoveZone
	// mutates the battlefield while we walk it (the attachmentSBAs discipline).
	if ev.Kind == events.Attach && ev.Obj != 0 && len(ev.IDs) > 0 {
		if attaching := e.G.Obj(ev.Obj); attaching != nil && isRole(attaching) {
			for _, p := range e.G.AliveFrom(0) {
				zone := append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, p)...)
				for _, id := range zone {
					o := e.G.Obj(id)
					if o == nil || id == ev.Obj || o.AttachedTo != ev.IDs[0] || !isRole(o) {
						continue
					}
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZBattlefield, To: state.ZGraveyard,
						Text: "another Role on it: the old Role goes to the graveyard"})
				}
			}
		}
	}
	if e.applyingReplacement {
		ev = events.CarryAction(e.replAction, e.replReplaced, ev)
	} else {
		replaced, handled := e.applyReplacements(ev)
		if handled {
			return replaced
		}
		// Not replaced, but possibly REWRITTEN in place (a DamageDone
		// ReplaceEffect body changed the amount): the returned event is what
		// gets logged, not the emit caller's copy.
		ev = replaced
	}
	// CR 306.8's planeswalker loyalty exchange (and CR 120.3e's exception for
	// a permanent that is also a creature) is folded directly into this
	// Damage event by events.Apply below -- AddCounter("LOYALTY", ...) runs
	// in the same Apply call that would otherwise mark damage, so replay
	// derives it from the one logged Damage event and no separate
	// CounterChange is ever emitted for it.
	// LKI (CR 603.10 "look back in time") is captured HERE, before
	// events.Emit runs Apply and mutates the object -- a zone-change trigger
	// needs the object exactly as it was a moment ago (its counters, tapped
	// state, damage, controller, zone), not the reset state Move leaves it
	// in. See effects.Ctx.LKI.
	var lki *state.Object
	var lkiPower, lkiToughness int32
	var lkiPTValid bool
	switch ev.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack:
		if o := e.G.Obj(ev.Obj); o != nil {
			cp := o.CloneDeep()
			lki = &cp
			if o.Zone == state.ZBattlefield && o.Face() != nil {
				lkiPower, lkiToughness = e.Power(o.ID), e.Toughness(o.ID)
				lkiPTValid = true
			}
		}
	}
	departingSource, departingSourceLifelink, departingSourceController := e.captureSourceLifelinkLKI(ev)
	stackLen := len(e.G.Stack)
	stored := events.Emit(e.G, e.L, ev)
	if len(e.turnsTaken) == len(e.G.Players) && e.turnsTakenEpoch == len(e.L.Events)-1 {
		if stored.Kind == events.TurnChange && int(stored.Player) < len(e.turnsTaken) {
			e.turnsTaken[stored.Player]++
		}
		e.turnsTakenEpoch++
	} else {
		e.turnsTaken = nil
		e.turnsTakenEpoch = 0
	}
	e.loop.observe(stored)
	if ev.Kind == events.StackCopy && len(e.G.Stack) > stackLen {
		copyID := e.G.Stack[len(e.G.Stack)-1]
		if tc, ok := e.triggerContexts[ev.Obj]; ok {
			e.triggerContexts[copyID] = tc
		}
		if lki, ok := e.triggerLKI[ev.Obj]; ok {
			if e.triggerLKI == nil {
				e.triggerLKI = make(map[state.ObjID]triggerObjectLKI)
			}
			if lki.object != nil {
				cp := lki.object.CloneDeep()
				lki.object = &cp
			}
			e.triggerLKI[copyID] = lki
		}
		if link, ok := e.sourceLifelinkLKI[ev.Obj]; ok {
			if e.sourceLifelinkLKI == nil {
				e.sourceLifelinkLKI = make(map[state.ObjID]bool)
			}
			e.sourceLifelinkLKI[copyID] = link
		}
		if controller, ok := e.sourceControllerLKI[ev.Obj]; ok {
			if e.sourceControllerLKI == nil {
				e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
			}
			e.sourceControllerLKI[copyID] = controller
		}
		if lki := e.damageSourceLKI[ev.Obj]; lki != nil {
			if e.damageSourceLKI == nil {
				e.damageSourceLKI = make(map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI)
			}
			e.damageSourceLKI[copyID] = cloneDamageSourceLKI(lki)
		}
	}
	if ev.Kind == events.MoveZone {
		// CR 400.7: an object that changes zones is a new object with no
		// memory of having become tapped this turn.
		delete(e.tappedTurn, ev.Obj)
	}
	if ev.Kind == events.MoveZone && ev.From == state.ZStack && ev.To != state.ZStack {
		delete(e.triggerContexts, ev.Obj)
		delete(e.triggerLKI, ev.Obj)
		delete(e.sacrificedLKI, ev.Obj)
		delete(e.sourceLifelinkLKI, ev.Obj)
		delete(e.sourceControllerLKI, ev.Obj)
		delete(e.damageSourceLKI, ev.Obj)
	}
	if ev.Kind == events.MoveZone && lki != nil && lki.Zone == state.ZBattlefield {
		// ChangeZone's Duration$ UntilHostLeavesPlay (the Oblivion Ring /
		// Banisher Priest pattern): the exiling permanent has just left the
		// battlefield, so every card it exiled under that duration and that is
		// still in exile returns to the zone it was exiled from, under its
		// owner's control. The sweep runs BEFORE this leave event's own
		// triggers are matched, matching Forge's command semantics (the return
		// is not a triggered ability); the returned cards' own ETB triggers
		// are queued by their return move's emit. events.Move prunes the
		// marker entries when their object leaves exile by any other path, so
		// the sweep can never return a card whose exile was another effect's
		// business, and a card exiled again by something else after it was
		// once returned is equally out of reach.
		if o := e.G.Obj(ev.Obj); o != nil && o.Zone != state.ZBattlefield {
			e.sweepExileReturn(ev.Obj)
		}
	}
	if ev.Kind == events.MoveZone {
		// Effect-created continuous effects' move-driven lifetimes (the
		// ForgetOnMoved$/ExileOnMoved$ sweep) run after the move is applied
		// and before this event's triggers are checked, so a may-play grant's
		// remembered set is already pruned when anything downstream reads it.
		// EVERY move, not just a battlefield departure: the move the sweep
		// exists for is a may-play cast's Exile→Stack move — which rides
		// PutOnStack, the event kind a cast pushes with — and a remembered
		// card can also leave the ForgetOnMoved$ zone from the graveyard or
		// the hand.
		e.effectMoveSweep(ev)
	}
	if ev.Kind == events.PutOnStack {
		e.effectMoveSweep(ev)
	}
	if ev.Kind == events.CounterChange {
		// Effect-created continuous effects' counter-driven lifetime (task
		// vow1; ForgetCounter$): after a counter REMOVAL is applied, a
		// remembered card whose count of the named kind reached zero leaves
		// the effect's Remembered set -- Promise of Loyalty's "for as long
		// as it has a vow counter on it". Applied before this event's own
		// triggers are checked, the same timing effectMoveSweep keeps.
		e.effectCounterSweep(ev)
	}
	// Damage batch (CR 510.4, Forge dealAssignedDamage): DamageDealtOnce/
	// DamageDoneOnce latch once per damage BATCH. A Damage event arriving with
	// no batch already open (combat's damageStep and effects' dealDamage calls
	// open their own; see Host.BeginDamageBatch) is a batch of one -- its own
	// batch, opened and closed around the trigger check, so the Once modes
	// fire per event rather than per turn and the queued referent's amount is
	// already the batch total. A prevented Damage never reaches here (emit
	// returned the prevention Note above), and the deferred cast-trigger arm
	// skips the trigger check entirely, so neither needs a batch.
	onlyEventBatch := false
	if ev.Kind == events.Damage && !e.damageBatchOpen {
		e.openDamageBatch()
		onlyEventBatch = true
	}
	if ev.Kind == events.PutOnStack && e.deferCastTrigger {
		// CR 601.2i: the cast trigger must not fire at the up-front push
		// (601.2a), because the spell is not yet cast -- targets (601.2c) and
		// payment (601.2h) still lie ahead. Hold the event and its LKI so
		// payCast's fireDeferredCastTrigger re-walks it after payment.
		// Deferring here (rather than queueing the trigger and removing it
		// later) means the held event is never double-checked: the first
		// pass skipped it entirely, and exactly one later pass fires it.
		cp, lp := stored, lki
		e.deferredPush, e.deferredPushLKI = &cp, lp
	} else {
		if _, _, loss := lifeLoss(stored); loss && e.lifeLossBatchDepth > 0 {
			e.lifeLossBatch = append(e.lifeLossBatch, stored)
		}
		e.checkTriggers(stored, lki, lkiPower, lkiToughness, lkiPTValid)
	}
	if onlyEventBatch {
		e.closeDamageBatch()
	}
	if ev.Kind == events.Tap && !e.tapIsEntryState(ev) {
		// Recorded after the triggers above were matched, so a FirstTime$
		// trigger sees whether an EARLIER tap happened this turn.
		if e.tappedTurn == nil {
			e.tappedTurn = make(map[state.ObjID]int32)
		}
		e.tappedTurn[ev.Obj] = e.G.Turn
	}
	e.finishSourceLifelinkLKI(ev, departingSource, departingSourceLifelink, departingSourceController)
	// CR 702.163 ("Start your engines!", rules/speed.go): a loss may raise
	// every eligible opponent's speed (if they have any), and a Start your
	// engines! permanent's battlefield entry starts a speed-less
	// controller's speed at 1. Checked on the FOLDED event, after
	// checkTriggers, so the gain event follows everything the loss itself
	// caused -- and both checks are inert for every other event. The loss
	// reaches here two ways: an explicit LifeChange with a negative amount
	// (life payment, "each player loses N life"), and a player D_DAMAGE --
	// combat damage and spell/ability damage fold straight to the life
	// total (events.Apply's Damage case) without a LifeChange, and any
	// Damage event that reaches emit has already been through prevention
	// (a prevented hit is a Note, never a Damage), so a positive player
	// Damage event here IS the life loss the rule reads.
	if (ev.Kind == events.LifeChange && ev.Amount < 0) ||
		(ev.Kind == events.Damage && ev.Obj == 0 && ev.Amount > 0) {
		e.checkSpeedGain(ev)
	}
	if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield {
		e.checkSpeedStart(ev.Obj)
	}
	// E2: any genuinely state-changing event proves the game is making
	// progress, so it clears the held-out cast suppression (suppressedCast,
	// see engine.go): a declined card's option comes back the moment the
	// game does anything else, which is exactly when ending the current
	// priority window (a spell resolves, a step or turn changes) and any
	// mana/board change both land. The four kinds a declined-Delve abort
	// emits -- a Priority regrant, the KChoose decision bookkeeping, and the
	// abort Note -- are excluded, so suppression survives only as long as
	// NOTHING else is happening, which is precisely the no-progress-interval
	// it is there to bound.
	if ev.Kind != events.Priority && ev.Kind != events.DecisionAsk &&
		ev.Kind != events.DecisionMade && ev.Kind != events.Note {
		e.suppressedCast = nil
		e.castAborts = nil
		// CR 611.2b: a "for as long as" control effect ends the moment its
		// condition stops holding, not at the next state-based check.
		e.expireControl(controlOnEvent)
		// A GainControl$ static (Mind Control) is realized the same way:
		// ending ran above (a static grant's grantEnded reads the fresh
		// wanted set), this registers the transfers the live scan newly
		// wants. Both are no-ops unless such a static is in play.
		e.reconcileControlStatics()
	}
	return stored
}

// sweepExileReturn implements ChangeZone's Duration$ UntilHostLeavesPlay
// return half: the object named by source has just left the battlefield, so
// every card it exiled under that duration and that is still in exile moves
// back to the zone it was exiled from, under its owner's control (events.Move
// gives a battlefield re-entry its owner's control, and the returned card's
// own ETB triggers queue through its return move's own emit). The marker list
// is snapshotted first: the return moves prune it underneath the loop. Entry
// order -- the order the exiles happened in -- is the return order,
// deterministic.
func (e *Engine) sweepExileReturn(source state.ObjID) {
	src := e.G.Obj(source)
	if src == nil || len(src.ExileReturn) == 0 {
		return
	}
	pending := append([]state.ExileReturnEntry(nil), src.ExileReturn...)
	for _, entry := range pending {
		o := e.G.Obj(entry.Obj)
		if o == nil || o.Zone != state.ZExile {
			continue
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: entry.Obj, From: state.ZExile, To: entry.From})
	}
}

// captureSourceLifelinkLKI preserves CR 608.2h's pre-departure derived
// lifelink state for every independent ability of the leaving permanent that
// already exists on the stack or in the pending-trigger queue. It is called
// immediately before every rules-layer events.Emit path that can fold a real
// MoveZone, including Updated replacement paths that deliberately bypass
// Engine.emit to avoid matching the same replacement twice. Walking ordered
// slices rather than a map keeps this bookkeeping incapable of changing event
// order. A self-sacrifice activation is not minted until after its departure;
// payCast captures that one sibling before paying the cost.
func (e *Engine) captureSourceLifelinkLKI(ev events.Event) (bool, bool, state.PlayerID) {
	if ev.Kind != events.MoveZone || ev.From != state.ZBattlefield ||
		ev.To == state.ZBattlefield {
		return false, false, 0
	}
	// A destruction batch's own pre-state wins (rules.Engine.BatchDepartures,
	// effects.Host): a later batch member must read the lifelink state from
	// immediately before the FIRST departure (CR 603.10a/702.15c -- the
	// destroy-all over a lifelink-granting Equipment and its bearer), not
	// the live layers an earlier member's departure already stripped. The
	// entry is consumed here; BatchDepartures rebuilds the map on its next
	// call, so a straggler for an object that never left cannot outlive one
	// effect call.
	link, batched := e.batchLifelink[ev.Obj]
	if batched {
		delete(e.batchLifelink, ev.Obj)
	} else {
		link = e.HasKeyword(ev.Obj, "Lifelink")
	}
	controller := e.G.Obj(ev.Obj).Controller
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil && o.Ability != nil && o.Source == ev.Obj {
			if e.sourceLifelinkLKI == nil {
				e.sourceLifelinkLKI = make(map[state.ObjID]bool)
			}
			if e.sourceControllerLKI == nil {
				e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
			}
			e.sourceLifelinkLKI[id] = link
			e.sourceControllerLKI[id] = controller
		}
		e.captureNamedDamageSourceLKI(id, ev.Obj, link, controller)
	}
	for i := range e.pendingTriggers {
		if e.pendingTriggers[i].Source == ev.Obj {
			e.pendingTriggers[i].Ctx.SourceLifelinkLKI = link
			e.pendingTriggers[i].Ctx.SourceLifelinkLKIValid = true
			e.pendingTriggers[i].Ctx.SourceControllerLKI = controller
			e.pendingTriggers[i].Ctx.SourceControllerLKIValid = true
		}
		e.capturePendingNamedDamageSourceLKI(&e.pendingTriggers[i].Ctx, ev.Obj, link, controller)
	}
	return true, link, controller
}

// finishSourceLifelinkLKI attaches the same pre-departure snapshot to a
// dies/leaves trigger that the event itself just queued. Such a trigger did not
// exist during captureSourceLifelinkLKI's pre-event walk.
func (e *Engine) finishSourceLifelinkLKI(ev events.Event, departing, link bool, controller state.PlayerID) {
	if !departing {
		return
	}
	for i := range e.pendingTriggers {
		if e.pendingTriggers[i].Source == ev.Obj {
			e.pendingTriggers[i].Ctx.SourceLifelinkLKI = link
			e.pendingTriggers[i].Ctx.SourceLifelinkLKIValid = true
			e.pendingTriggers[i].Ctx.SourceControllerLKI = controller
			e.pendingTriggers[i].Ctx.SourceControllerLKIValid = true
		}
		e.capturePendingNamedDamageSourceLKI(&e.pendingTriggers[i].Ctx, ev.Obj, link, controller)
	}
}

func (e *Engine) captureNamedDamageSourceLKI(stack, source state.ObjID, link bool, controller state.PlayerID) {
	if e.damageSourceLKI == nil {
		e.damageSourceLKI = make(map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI)
	}
	if e.damageSourceLKI[stack] == nil {
		e.damageSourceLKI[stack] = make(map[state.ObjID]effects.DamageSourceLKI)
	}
	e.damageSourceLKI[stack][source] = effects.DamageSourceLKI{Lifelink: link, Controller: controller}
}

func (e *Engine) capturePendingNamedDamageSourceLKI(ctx *effects.Ctx, source state.ObjID, link bool, controller state.PlayerID) {
	if ctx.DamageSourceLKI == nil {
		ctx.DamageSourceLKI = make(map[state.ObjID]effects.DamageSourceLKI)
	}
	ctx.DamageSourceLKI[source] = effects.DamageSourceLKI{Lifelink: link, Controller: controller}
}

func cloneDamageSourceLKI(in map[state.ObjID]effects.DamageSourceLKI) map[state.ObjID]effects.DamageSourceLKI {
	out := make(map[state.ObjID]effects.DamageSourceLKI, len(in))
	for id, lki := range in {
		out[id] = lki
	}
	return out
}

func (e *Engine) Pending() *decision.Decision { return e.pending }

// seatFacingName is the seat-facing identity for client-facing prompt and
// option-label text (the priority prompt, the keep/mulligan prompt, the
// attacker, cumulative-upkeep and target option labels). PlayerName is
// supplied by the table and identifies a human even when two players chose
// the same deck; Name is the deterministic fallback for bots or callers
// without display names. Decision prompts and option labels are NOT chain
// content (rules/engine.go's ask emits only DecisionAsk{Kind}), so
// composing them from PlayerName moves no chain head — but event text must
// stay on Name (the F3 invariant, rules/playername_test.go). The final
// fallback is defensive: decision seats originate from AliveFrom, but
// malformed state must not panic while constructing a client decision.
func seatFacingName(g *state.Game, p state.PlayerID) string {
	if g != nil && int(p) < len(g.Players) {
		if name := g.Players[p].PlayerName; name != "" {
			return name
		}
		if name := g.Players[p].Name; name != "" {
			return name
		}
	}
	return fmt.Sprintf("seat %d", p)
}

// BeginLibrarySearch marks the start of one library search's execution: the
// searcher's repl:Moved FoundSearchingLibrary$ replacements apply to exactly
// the moves the search emits.
func (e *Engine) BeginLibrarySearch(owner state.PlayerID) {
	e.searchDepth++
	e.searchingBy = owner
}

// EndLibrarySearch closes the innermost search scope.
func (e *Engine) EndLibrarySearch() {
	if e.searchDepth > 0 {
		e.searchDepth--
	}
	if e.searchDepth == 0 {
		e.searchingBy = 0
	}
}

// searchControlRedirect applies the ControlOpponentsSearchingLibrary$ static
// family (Opposition Agent's "You control your opponents while they're
// searching their libraries"): the search pick's decision is posed to the
// static's controller instead of the searching player. Scope: the pick (and
// any later ask the search flow poses) — the "control" of the searched
// player's every action is the wider grant Forge models; this build
// redirects the decisions the search itself asks, which is what a
// resolution can observe. The first static in activeStatics' deterministic
// APNAP order wins; an Affected$ spec that does not match the searching
// player leaves the static inert.
func (e *Engine) searchControlRedirect(d *decision.Decision) {
	if d.ResumeKind != "search" {
		return
	}
	for _, sv := range e.activeStatics("Continuous") {
		if strings.TrimSpace(sv.Params["ControlOpponentsSearchingLibrary"]) != "You" {
			continue
		}
		if sv.Controller == d.Player {
			continue
		}
		if spec := sv.Params["Affected"]; spec != "" &&
			!effects.MatchesPlayerSpec(e.G, spec, d.Player, sv.Controller) {
			continue
		}
		d.Player = sv.Controller
		return
	}
}

func (e *Engine) ask(d *decision.Decision) {
	e.searchControlRedirect(d)
	// Empty-answer-only tripwire (the class the Squadron Hawk fail-to-find
	// search wedged): a decision whose ONLY legal answer is the empty one
	// (Min 0 with Max 0, or no options at all) can never be answered
	// differently by any seat, so posing it strands the game on an ask a
	// client has no control to send. Every asking primitive in effects goes
	// through effects.Ask, which refuses to post the shape and resolves it
	// silently instead; this boundary guard is what fails a test loudly if
	// any construction site -- here or a future one -- ever posts one
	// anyway. Panic rather than quietly fixing: by the time a decision
	// reaches ask the asking caller has already chosen its resolution path,
	// and silently swallowing it here would leave the caller's suspended
	// half-resolution dangling.
	// One mid-resolution decision at a time (the structural guard for the
	// overwrite class findings-sol4 proved on the life-replacement draw
	// loop): a caller reaching ask while a SUSPENDED RESOLUTION is awaiting
	// its answer would overwrite e.pending and orphan that resolution's
	// ask -- no seat can ever answer it, and the surviving ask's answer is
	// applied to the wrong resolution. Every guarded caller checks
	// e.pending/e.Suspended() before asking (effDraw's cursor loop,
	// lifeReplacementDraw's park, advanceStep's draw-step return, the
	// replacement-choice queue's askNextReplacementChoice, DrawFor's
	// suspended degrade); a caller that does not is a bug of exactly the
	// class those guards exist for. Panic, the same stance as the two
	// checks below: the host crashes the match loudly rather than shipping
	// a log with a decision nobody can answer.
	//
	// A pending decision with e.resume == nil is deliberately NOT a panic,
	// and neither is a pose with e.resume != nil but e.pending == nil: the
	// replacement-order flow (handleReplacement) parks its resume point
	// while it finishes the parked event's remaining work -- a combat pass's
	// completeCombatPass then poses the round's priority with e.pending nil,
	// nothing is displaced, and the parked frame resumes when the engine
	// returns to it. What the guard exists for is the OVERWRITE: an ask
	// displacing a decision a seat has not answered yet. In engine flow
	// nothing emits while such a decision is outstanding (Advance is parked
	// on it and handle runs only after Submit cleared it), but the test
	// probes drive e.emit directly while a setup priority ask is pending,
	// and an emit that poses an ask is then the probe's intent -- the
	// priority ask it displaces is re-granted by the same Submit tail. That
	// displacement is engine-unreachable and probe-owned.
	if e.resume != nil && e.pending != nil {
		panic(fmt.Sprintf("rules: ask overwrote a suspended resolution's pending decision (%s, seat %d) with %s for seat %d",
			e.pending.Kind, e.pending.Player, d.Kind, d.Player))
	}
	if effects.OnlyEmptyAnswer(d) {
		panic(fmt.Sprintf("rules: decision %s for seat %d posed with only the empty answer legal (Min %d Max %d, %d options) -- asking primitives must resolve this shape silently (effects.Ask), never post it",
			d.Kind, d.Player, d.Min, d.Max, len(d.Options)))
	}
	// Option.Index/position identity (finding bi). Every decision that can
	// reach a seat flows through ask -- ask is what sets d.Seq and e.pending,
	// so a decision that skipped it is not pending and no seat can answer it
	// -- which makes this the ONE place the invariant is enforced for every
	// construction site at once, including the sites that never call
	// decision.New (all but mulligan's two build the struct literal directly;
	// effects' two mid-resolution asks reach e.pending through Engine.Ask,
	// which calls back into ask below). A mis-indexed list is a programming
	// error, not a bad client answer: Chosen resolves an intent by position,
	// so an option whose Index has drifted off its slot makes the engine
	// resolve a different option than the client named, silently. Panic here
	// rather than return an error because an error hands the caller the
	// choice to swallow it and ship the broken Decision to a seat -- exactly
	// the silent wrong-option failure the invariant exists to make
	// unrepresentable.
	for i := range d.Options {
		if d.Options[i].Index != i {
			panic(fmt.Sprintf("rules: decision option %d has Index %d, want position %d (%s)",
				i, d.Options[i].Index, i, d.Kind))
		}
	}
	d.Seq = uint64(len(e.L.Events))
	e.emit(events.Event{Kind: events.DecisionAsk, Player: d.Player, Text: string(d.Kind)})
	e.pending = d
}

// Advance runs engine work until a decision is required or the game ends.
func (e *Engine) Advance() {
	for !e.G.Over && e.pending == nil {
		e.step()
	}
}

// Submit applies a client's answer. Anything the engine did not offer is
// rejected, which is what keeps the client rules-ignorant.
func (e *Engine) Submit(in decision.Intent) error {
	if e.G.Over {
		return fmt.Errorf("game is over")
	}
	d := e.pending
	if d == nil {
		return fmt.Errorf("no decision pending")
	}
	if err := d.Validate(in); err != nil {
		return err
	}
	if d.Kind == decision.KAttackers {
		// Ruling m34: the KAttackers option list offers every (attacker,
		// defender) pair, so an intent naming the same creature twice --
		// against two different defenders -- passes Validate's per-index
		// checks while declaring it attacking two players (CR 506.2).
		// Reject before the intent is recorded and the decision consumed,
		// so the pending attackers decision survives for a legal answer.
		if err := e.validateAttackers(d, in); err != nil {
			return err
		}
	}
	if d.Kind == decision.KBlockers {
		// CR 509.1a: the blockers option list similarly offers every legal
		// (blocker, attacker) pair. Reject choosing the same ordinary blocker
		// against multiple attackers while preserving the pending decision.
		if err := e.validateBlockers(d, in); err != nil {
			return err
		}
	}
	if d.Kind == decision.KChoose {
		// The cast flow's Convoke/Harmonize announcement (convokeAsk): the
		// static option list cannot express "only while the outstanding
		// cost can still absorb the contribution", so an over-selection
		// (two white creatures for one {W}) passes Validate's per-index and
		// group checks. Reject it here, before the intent is recorded and
		// the pending decision consumed, so a legal subset can be
		// resubmitted -- the same preserve-and-reject shape as
		// validateAttackers above.
		if err := e.validateCastContributions(d, in); err != nil {
			return err
		}
		// ShareLandType$ True (Myriad Landscape): a hidden-library search's
		// answer must name cards that all share one land type -- the option
		// list cannot express the pairwise constraint, so an answer naming
		// e.g. a Forest and a Mountain is rejected and the pending decision
		// survives for a legal (or smaller) answer. Single-card answers are
		// trivially legal.
		if err := e.validateSearch(d, in); err != nil {
			return err
		}
	}
	if d.Kind == decision.KTarget {
		// TargetsWithSameController$ True (Lodestone Bauble): a target
		// announcement's answer must name objects that all share one owner —
		// in a graveyard, the "controller" a card in a graveyard has. The
		// offered option list spans every player's graveyard, a pairwise
		// constraint the option shape cannot express, so the answer is
		// rejected here (the validateSearch preserve-and-reject shape) and
		// the pending decision survives for a legal (or smaller) answer.
		if err := e.validateSameControllerTargets(d, in); err != nil {
			return err
		}
	}
	e.L.Intents = append(e.L.Intents, in)
	e.emit(events.Event{Kind: events.DecisionMade, Player: in.Player,
		Text: fmt.Sprintf("%s:%v", d.Kind, in.Choices)})
	e.pending = nil
	e.handle(d, in)
	// CR 704.4: nobody receives priority in the middle of a resolution. A
	// handler may have resumed an effect only far enough to pose another
	// mid-resolution decision; in that case state-based actions wait until
	// the resolution finishes. The answer that finishes it clears resume, so
	// this same boundary performs the deferred check before Advance can grant
	// priority.
	if !e.Suspended() {
		e.checkStateBased()
	}
	e.Advance()
	return nil
}

// drawCard draws for the turn structure, sharing effects.DrawFor with the
// Draw primitive so the draw step and a card that says "draw a card" can
// never disagree about what drawing means.
//
// Ruling T22-d: this used to call checkGameOver alone -- correct as far as
// it went (an empty-library draw is itself a loss, via the PlayerLost
// DrawFor emits directly), but it skipped destroyLethalDamage and, now,
// checkLoseConditions' own permanent-removal sweep. checkStateBased runs
// both of those in addition to checkGameOver, so a player who decks out
// here has their battlefield cleaned up the same way one who hits 0 life
// does, rather than only on whatever later checkStateBased call happens to
// come next.
func (e *Engine) drawCard(p state.PlayerID) {
	effects.DrawFor(e, p)
	e.checkStateBased()
}

// cardsKeywordHead lets layers.go strip a keyword's parameters ("Equip:2" ->
// "Equip") without importing cards itself for one call.
func cardsKeywordHead(k string) string { return cards.KeywordHead(k) }
