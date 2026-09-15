// Package effects implements the primitives Forge card scripts reference. It
// reaches the engine through the Host interface, so it never imports rules and
// the dependency graph stays acyclic.
package effects

import (
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Host is everything an effect may do to a game: read it, and propose events.
// Deliberately tiny — an effect that needs more is a sign the primitive is
// doing rules work that belongs in the rules package.
type Host interface {
	// Game returns the live match state for reading. The returned *state.Game
	// must never be written to directly: every state mutation goes through
	// Emit (which routes through events.Apply), which is what keeps the event
	// log a complete description of the match.
	Game() *state.Game
	Emit(events.Event)
	// EmitTap taps the permanent obj with the synchronous provenance a Taps
	// trigger reads but the replayed Tap event does not carry: tapper is the
	// player who tapped it (Forge Card.tap's tapper -- the resolving
	// ability's activator, a cost's payer), and entering marks a permanent
	// being given its tapped entry state by a DB$ Tap | ETB$ True replacement
	// body, which CR 603.2e says never "becomes tapped" (Forge TapEffect's
	// ETB branch sets the state without running Taps triggers). The emitted
	// event is exactly Emit(events.Event{Kind: events.Tap, Obj: obj}), so the
	// hash chain is unaffected; rules.Engine keeps the provenance as event
	// context while the event's triggers are matched.
	EmitTap(obj state.ObjID, tapper state.PlayerID, entering bool)
	// Rand is the engine's seeded generator. Effects that need randomness must
	// use it and nothing else, or replay breaks.
	Rand(n int) int
	// AddContinuous registers one continuous effect against the CR 613 layer
	// system (rules.Engine.AddContinuous). This is how Pump, PumpAll, Animate
	// and Protection reach the layer system without effects importing rules,
	// which would be an import cycle (effects sits below rules). Task 19c.
	AddContinuous(state.ContinuousEffect)
	// RegisterControl records one GainControl effect with the lifetime its
	// LoseControl$ names (CR 611.2b "for as long as", CR 514.2 end of turn),
	// so the engine can end it through a ControlChange event the moment that
	// duration ends. A grant with no duration is permanent: it supersedes
	// every earlier control effect on the object (CR 613.7 timestamp order).
	RegisterControl(ControlGrant)
	// LegalTargets returns the targets the rules engine would offer for sa.
	// Redirect effects use this shared census rather than duplicating target
	// legality below rules (protection and continuous restrictions included).
	LegalTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA) []state.Target
	// RegenerationDisallowed reports whether an Effect-registered
	// CantRegenerate restriction makes id unable to be regenerated (Incinerate's
	// "can't be regenerated this turn"). Consulted by ReplaceDestruction before
	// it would consume a shield, so a banned regeneration is never honoured.
	// Implemented by rules.Engine against its continuous-effect registry; the
	// effects test double reports false (no engine to consult). Task ce1.
	RegenerationDisallowed(id state.ObjID) bool
	// HasKeyword reports a DERIVED keyword — printed or granted by a
	// continuous effect (rules.Engine.HasKeyword). Effects that gate on a
	// keyword (Destroy on Indestructible) must ask this, never the face.
	HasKeyword(id state.ObjID, kw string) bool
	// Power, Toughness and IsCreature are current derived characteristics.
	// Damage/count effects must not read a printed face when layers modify P/T
	// or make a planeswalker a creature.
	Power(id state.ObjID) int32
	Toughness(id state.ObjID) int32
	IsCreature(id state.ObjID) bool
	// CastThisTurn counts the spells cast this turn by anyone, derived from
	// the event log so a replay that rebuilds the game arrives at the same
	// number (Task 17's Count$ThisTurnCast backing — a copy/Storm count
	// must be replay-derivable, never a live-only engine counter).
	CastThisTurn() int
	// Ask poses a decision in the middle of a resolution. It sets the host's
	// pending decision, sets the mid-resolution resume state, and returns
	// true. A true return tells the calling effect to stop and wait: the
	// resolution is suspended, and the answered decision re-enters it
	// (rules' Engine.Ask is what runs the resume continuation). false means
	// the host cannot ask now — an effects-package test double, or a
	// rules-internal context with no engine to drive — and the calling
	// effect falls back to its deterministic stand-in (R-9). M2d-2.
	Ask(d *decision.Decision) bool
	// Suspended reports whether the resolution is currently suspended on a
	// mid-resolution ask — Ask returned true and set the host's resume state,
	// which has not yet been cleared by the answer arriving. effects.Resolve
	// calls it after every sub-ability in a chain so that a suspended ask
	// STOPS the chain rather than running the sub-abilities beneath it (B1: a
	// chained SA such as Thoughtseize's Discard | SubAbility$ DBLoseLife must
	// not fire its SubAbility on the initial pass, before the answer exists,
	// nor again on the resume pass — the resume re-enters at the asking SA
	// and walks the rest of the chain exactly once). An effects-package test
	// double that cannot suspend reports false; the rules engine reports
	// e.resume != nil, which Ask sets and the handled answer clears.
	Suspended() bool
	// SuspendContinuation reports that a Resolve loop has just suspended (its
	// host reported Suspended() after running the sub-ability at sa) and is
	// about to return, so the chain it was walking must resume at sa.Sub once
	// the pending answer and any deeper continuations are done. effects.Resolve
	// calls it at EVERY loop level that suspends, including the innermost
	// (the level whose sa is the pending ask's ResumeSA): a host records the
	// enclosing levels (sa != the pending ResumeSA) as outer continuations and
	// drops the innermost one, because re-entering the pending ask's own SA
	// already walks sa.Sub. A host that never suspends (an effects-package
	// double, where Ask returns false) never sees this call.
	SuspendContinuation(sa *cards.SA)
	// SuspendRepeat reports that one iteration of a RepeatEach loop suspended
	// at a mid-resolution ask. The host must bind the suspended iteration's
	// Remembered to the pending ask (and to the iteration's own continuation
	// frames) and resume the loop at the next subject once they complete,
	// rather than dropping the remaining subjects (CR 608.2c). The Resolve
	// loop enclosing the RepeatEach reports that SA through
	// SuspendContinuation next; the host drops that report, because the loop
	// frame re-enters the RepeatEach itself and so walks its Sub.
	SuspendRepeat(RepeatSuspension)
	// SetDamageSource overrides the in-flight damage source for the Damage
	// events the caller is about to emit: the provenance rules' emit-side
	// protection check (CR 702.16d) and DamageDone trigger matching read
	// for every Damage event. It returns the previous override so the
	// caller restores it before returning; zero restores "no override".
	// The override is engine-transient state exactly like the resolution
	// source it wraps: replay re-executes the same setter, and Clone never
	// copies it because an emitter always restores before returning
	// (DealDamage/DamageAll never ask mid-loop, so nothing suspends inside
	// the override window).
	SetDamageSource(id state.ObjID) state.ObjID
	// BatchDepartures declares that the caller is about to emit MoveZone
	// events for every object in ids as one simultaneous destruction batch
	// (CR 704.3): the engine snapshots each object's derived lifelink
	// state NOW, before any of the moves fold, so a later batch member's
	// CR 603.10a departure capture reads the batch's own pre-state rather
	// than whatever an earlier member's departure already stripped (a
	// destroy-all over a lifelink-granting Equipment and its bearer: the
	// bearer's lifelink LKI must not depend on battlefield order). Entries
	// are consumed by the matching departure capture. EndBatchDepartures
	// clears any remaining entry after the effect loop, including a member
	// regeneration kept on the battlefield.
	BatchDepartures(ids []state.ObjID)
	EndBatchDepartures()
}

// RepeatCursor is a RepeatEach loop re-entered after an iteration suspended:
// the subjects captured when the loop started (never re-derived mid-loop),
// the index of the next subject, and the completed iteration's final
// Remembered so the objects that iteration remembered outlive it.
type RepeatCursor struct {
	SA       *cards.SA
	Subjects []state.Target
	Next     int
	Last     []state.Target
	HasLast  bool
}

// RepeatSuspension is what effRepeatEach reports when an iteration asks.
// Body is the suspended iteration's Remembered (the loop subject plus
// anything the iteration remembered before asking); Outer and Chosen are the
// RepeatEach resolution's own bindings, restored when the loop re-enters.
type RepeatSuspension struct {
	RepeatCursor
	Body        []state.Target
	Outer       []state.Target
	Chosen      []state.Target
	ChosenValid bool
}

// Ctx carries the bindings a Forge script refers to during resolution.
type Ctx struct {
	TriggerContext
	Source     state.ObjID
	Controller state.PlayerID
	Targets    []state.Target
	Remembered []state.Target
	// Captured is the part of Remembered the resolution started with because
	// its trigger, delayed trigger or replacement put the event's object there
	// (this engine's stand-in for Forge's separate TriggeredCard), rather than
	// because a Remember* parameter of the resolution chose it. Forge keeps
	// neither in a host's remembered list, so a RepeatEach over players does
	// not carry these into its iterations.
	Captured []state.Target
	// SourceLifelinkLKI is the source permanent's derived lifelink state at
	// the last moment it existed on the battlefield. The validity bit is
	// separate because "it did not have lifelink" is authoritative LKI too.
	// Rules seeds this on independently resolving abilities; damage uses it
	// only after the source has departed, and continues to read the live
	// derived source while it remains a permanent.
	SourceLifelinkLKI      bool
	SourceLifelinkLKIValid bool
	// Sacrificed carries the last-known-information snapshot of every object
	// this resolving spell/ability sacrificed, as it was at the instant of the
	// sacrifice (state.SacrificedInfo). Built two ways, feeding one field: a
	// cost-paid sacrifice carries it onto the stack object (rules/cast.go
	// commitCast) and resolution loads it here, while an effect-driven
	// sacrifice (effSacrifice with RememberSacrificed$ True) appends here
	// directly so a SubAbility$ chained after it can read it. The
	// Sacrificed$<Property> heads in count.go read it.
	Sacrificed []state.SacrificedInfo
	// SVars is the resolving card's SVar table, and X the value paid for {X}.
	// Both are bound by the rules package when it builds the context.
	SVars map[string]string
	X     int32
	// Replaced is the object the replaced event was about (Defined$ ReplacedCard):
	// the card a "would go to the graveyard from anywhere, exile it instead"
	// replacement is acting ON. Set by rules/replacement.go on the context it
	// builds for a matching ReplaceWith$; zero outside a replacement, and nil for
	// a zero (or gone) object when Defined resolves it. It is context, not state
	// -- it drives the replacement's own resolution but is never itself persisted
	// to the event log.
	Replaced state.ObjID
	// LKI is the object a zone-change trigger fired for, as it was just
	// before the move (CR 603.10 "look back in time"): Move resets counters,
	// tapped state and damage on the way out, so a "dies" condition such as
	// Undying's "if it had no +1/+1 counters" must read this, not the live
	// object. nil for every other trigger.
	LKI *state.Object
	// LKIPower/LKIToughness are that snapshot's derived battlefield P/T,
	// captured before the move removes continuous effects. The validity bit
	// distinguishes a real zero from a non-battlefield/no-characteristic LKI.
	LKIPower, LKIToughness int32
	LKIPTValid             bool
	// Modes is the answered modal choice on a re-entered mid-resolution
	// resolution (M2d-2): the SVar names of the chosen Choices$ sub-abilities,
	// in execution order. rules' resumeResolution sets it from the recorded
	// answer before re-running the suspended sub-ability, so effCharm's
	// re-entry runs exactly the chosen modes instead of asking again. Nil on
	// the first pass and on any non-modes resume.
	Modes []string
	// UnlessPay is the answered unless-pay choice on a re-entered
	// mid-resolution resolution (M2d-2): "pay" means rules' resumeResolution
	// has already paid the UnlessCost$ from the payer's pool and the asking
	// effect proceeds with its body; "decline" means it proceeds as if the
	// player declined (no effect). "" on the first pass, where the effect
	// poses the ask instead.
	UnlessPay string
	// Discard is the answered "Mode$ RevealYouChoose" discard choice on a
	// re-entered mid-resolution resolution: the object(s) the caster named
	// to be discarded from the target's hand. rules' resumeResolution sets
	// it from the recorded answer before re-running the suspended sub-ability,
	// so effDiscard's re-entry discards exactly the chosen cards instead of
	// asking again. Nil on the first pass and on any non-discard resume. The
	// ask itself carries the caster as the chooser and the target as the
	// discarder, which is why a plain ObjID is not enough state to rebuild:
	// the two player roles are re-derived from Ctx on re-entry.
	Discard []state.ObjID
	// Choice is the selected card(s) or player(s) from ChooseCard,
	// ChoosePlayer, or ChangeTargets. ChoiceDone distinguishes an answered
	// empty optional choice from its first pass.
	Choice     []state.Target
	ChoiceDone bool
	// ChoiceTarget is the index of the per-player chooser currently being
	// resumed. It keeps multi-player ChooseCard/ChoosePlayer asks from
	// returning to the first chooser after every answer.
	ChoiceTarget int
	// Chosen holds card/player choices for the remaining resolution chain.
	// Unlike Choice it is not the transport for a pending answer; filters such
	// as Creature.nonChosenCard consult it after ChooseCard has returned.
	Chosen      []state.Target
	ChosenValid bool
	// Repeat is set only on the re-entry of a suspended RepeatEach loop; the
	// RepeatEach whose SA it names consumes and clears it.
	Repeat *RepeatCursor
	// Search is the answered hidden-library KChoose selection on a re-entered
	// ChangeZone resolution. SearchDone distinguishes "answered with no cards"
	// from the first pass; Search preserves the player's answer order. The
	// asking effect consumes and clears both before continuing, so a nested
	// search cannot inherit the outer answer.
	Search     []state.ObjID
	SearchDone bool
	// Dig is the answered Dig look-and-take pick on a re-entered mid-resolution
	// resolution: the object(s) the library's owner picked out of the top
	// DigNum$ window to move to DestinationZone$, in the player's answer
	// order. rules' resumeResolution sets it from the recorded answer before
	// re-running the suspended sub-ability, so effDig's re-entry moves exactly
	// the chosen cards instead of asking again; DigDone distinguishes
	// "answered (possibly with no cards)" from the first pass, and DigTarget
	// identifies the Defined$ target whose library posed that ask. Re-entry
	// skips earlier targets (already processed before suspension), applies the
	// answer at DigTarget, then preserves the former deterministic processing
	// for later targets until per-library chained asks exist. The asking effect
	// consumes and clears all three fields at the top of its own walk (the fx42
	// scoping discipline), so a nested Dig cannot inherit the outer answer.
	Dig       []state.ObjID
	DigDone   bool
	DigTarget int
	// Arrange is the answered KArrange decision on a re-entered
	// mid-resolution resolution (Ruling J0): true once rules' handleArrange
	// has applied the answered arrangement and emitted the LibraryOrder
	// event, so effRearrangeTopOfLibrary's re-entry lets the resolution
	// continue (the chained SubAbility$ runs) instead of re-asking. False on
	// the first pass, where the effect poses the ask. The arrangement itself
	// lives on the LibraryOrder event, not on Ctx -- the answer shape is
	// applied by the rules handler, unlike Modes/UnlessPay/Discard where the
	// effect re-reads the answer -- so the field is only a done-marker.
	Arrange bool
	// RevealOpt is the answered RevealOptional$ yes/no on a re-entered
	// mid-resolution reveal (task fb-3f1cc033, the Delver of Secrets
	// PeekAndReveal shape): "yes" means the peeking player chose to reveal
	// (the Note is emitted, RememberRevealed$ fires) and "no" means they
	// declined (no Note, Remembered unchanged). "" on the first pass, where
	// the effect poses the ask (or, when the host cannot ask, falls back to
	// the mandatory reveal — the same R-9 degradation Scry/Surveil carry).
	// effReveal consumes and clears it before continuing, so a nested
	// RevealOptional$ peek in the same walk poses its own ask (fx42
	// scoping).
	RevealOpt string
}

type Effect func(h Host, c *Ctx, sa *cards.SA)

// atomicMap is a copy-on-write string-keyed map. Writes are rare — native
// primitives register themselves from init() and the only other writer is
// M3's plugin tier overriding one at runtime — so they pay the cost of taking
// a mutex and copying the snapshot. Reads are the hot path: Resolve does one
// lookup per effect resolution, on every match, in its own goroutine, so
// readers do a single atomic load and never block or contend with a writer or
// each other. A writer never mutates a map a reader might already hold: it
// always builds a fresh map and swaps the pointer.
type atomicMap[V any] struct {
	mu  sync.Mutex
	ptr atomic.Pointer[map[string]V]
}

func newAtomicMap[V any]() *atomicMap[V] {
	a := &atomicMap[V]{}
	m := map[string]V{}
	a.ptr.Store(&m)
	return a
}

func (a *atomicMap[V]) load() map[string]V { return *a.ptr.Load() }

// set installs or replaces one entry.
func (a *atomicMap[V]) set(key string, val V) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old)+1)
	for k, v := range old {
		next[k] = v
	}
	next[key] = val
	a.ptr.Store(&next)
}

// setAll installs or replaces several entries as a single atomic publish.
func (a *atomicMap[V]) setAll(kv map[string]V) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old)+len(kv))
	for k, v := range old {
		next[k] = v
	}
	for k, v := range kv {
		next[k] = v
	}
	a.ptr.Store(&next)
}

// delete removes the given keys, if present.
func (a *atomicMap[V]) delete(keys ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	old := *a.ptr.Load()
	next := make(map[string]V, len(old))
	for k, v := range old {
		next[k] = v
	}
	for _, k := range keys {
		delete(next, k)
	}
	a.ptr.Store(&next)
}

var registry = newAtomicMap[Effect]()

// Register installs an implementation for a Forge API name. Called from init
// functions in this package; re-registering replaces, which is what lets the
// plugin tier in M3 override a native primitive. Safe to call concurrently
// with Resolve and Supported (and with itself).
func Register(api string, fn Effect) { registry.set(api, fn) }

func unregister(apis ...string) { registry.delete(apis...) }

// Supported reports the primitive set this build implements, in the same
// prefixed form cards.Face.Primitives uses, so it feeds straight into
// cards.Registry.Coverage.
func Supported() map[string]bool {
	reg := registry.load()
	non := supportedNonAPI.load()
	out := make(map[string]bool, len(reg)+len(non))
	for k := range reg {
		out["api:"+k] = true
	}
	for k := range non {
		out[k] = true
	}
	return out
}

// supportedNonAPI holds keyword, trigger, static and replacement primitives,
// which are implemented in rules rather than as effect functions. Tasks 18-20
// fill it in.
var supportedNonAPI = newAtomicMap[bool]()

// RegisterNonAPI records a keyword, trigger, static or replacement primitive as
// implemented. The name must carry its prefix, e.g. "kw:Flying". Safe to call
// concurrently with Resolve and Supported (and with itself).
func RegisterNonAPI(prefixed ...string) {
	kv := make(map[string]bool, len(prefixed))
	for _, p := range prefixed {
		kv[p] = true
	}
	supportedNonAPI.setAll(kv)
}

const maxChain = 32

// Ask poses d through the host unless it offers nothing to choose. A
// decision with no options cannot be answered meaningfully (a seat can only
// submit the empty answer), so the asking effect takes its no-host path --
// the same result an answered empty choice produces -- without a pending
// decision or a resume. Effects should ask through this rather than
// h.Ask directly.
func Ask(h Host, d *decision.Decision) bool {
	if d == nil || len(d.Options) == 0 {
		return false
	}
	return h.Ask(d)
}

// Resolve runs an ability and every sub-ability chained beneath it.
func Resolve(h Host, c *Ctx, sa *cards.SA) {
	reg := registry.load()
	for d := 0; sa != nil && d < maxChain; d, sa = d+1, sa.Sub {
		// Condition* gate (task fb-3f1cc033): a sub whose supported condition
		// is evaluated and not met is skipped and the chain continues. An
		// unresolved shape (supported=false) runs unconditionally, the
		// documented pre-gate behaviour — see conditions.go for the exact
		// boundary and the counts behind it. A RepeatEach re-entered at its
		// loop cursor already passed its gate when the loop began; its
		// remaining iterations are part of that same resolution.
		resumingLoop := c.Repeat != nil && c.Repeat.SA == sa
		if !resumingLoop {
			if met, supported := conditionMet(h, c, sa); supported && !met {
				continue
			}
		}
		fn, ok := reg[sa.API]
		if !ok {
			// Unimplemented primitives must be loud but harmless: deck-build
			// validation is supposed to have caught this already.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API " + sa.API})
			continue
		}
		fn(h, c, sa)
		if h.Suspended() {
			// A sub-ability in this chain posed a mid-resolution ask and
			// suspended the resolution: do NOT descend into the rest of the
			// chain. The B1 bug was that this loop kept walking sa.Sub
			// unconditionally, so a chained SA ran its SubAbility$ on the
			// initial pass (before the answer existed) AND again when the
			// answered decision re-entered at the asking SA — Thoughtseize's
			// Discard | SubAbility$ DBLoseLife lost 4 life instead of 2. The
			// resume re-enters at THIS asking SA (rules' resumeResolution),
			// which re-runs the asking effect to apply the answer and then
			// continues walking sa.Sub exactly once.
			//
			// Report this loop's suspension point to the host so a NESTED ask
			// (an ask posed from inside this loop's own effect, e.g. the mode
			// a Charm runs) does not lose the chain this loop was still
			// carrying — fx32's defect. The host keeps the enclosing levels as
			// outer continuations and drops this one when it is the asking
			// loop's own level, which re-enters sa.Sub itself.
			h.SuspendContinuation(sa)
			return
		}
	}
}
