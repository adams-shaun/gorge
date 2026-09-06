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
	// Rand is the engine's seeded generator. Effects that need randomness must
	// use it and nothing else, or replay breaks.
	Rand(n int) int
	// AddContinuous registers one continuous effect against the CR 613 layer
	// system (rules.Engine.AddContinuous). This is how Pump, PumpAll, Animate
	// and Protection reach the layer system without effects importing rules,
	// which would be an import cycle (effects sits below rules). Task 19c.
	AddContinuous(state.ContinuousEffect)
	// HasKeyword reports a DERIVED keyword — printed or granted by a
	// continuous effect (rules.Engine.HasKeyword). Effects that gate on a
	// keyword (Destroy on Indestructible) must ask this, never the face.
	HasKeyword(id state.ObjID, kw string) bool
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
}

// Ctx carries the bindings a Forge script refers to during resolution.
type Ctx struct {
	Source     state.ObjID
	Controller state.PlayerID
	Targets    []state.Target
	Remembered []state.Target
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

// Resolve runs an ability and every sub-ability chained beneath it.
func Resolve(h Host, c *Ctx, sa *cards.SA) {
	reg := registry.load()
	for d := 0; sa != nil && d < maxChain; d, sa = d+1, sa.Sub {
		fn, ok := reg[sa.API]
		if !ok {
			// Unimplemented primitives must be loud but harmless: deck-build
			// validation is supposed to have caught this already.
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
				Text: "unimplemented API " + sa.API})
			continue
		}
		fn(h, c, sa)
	}
}
