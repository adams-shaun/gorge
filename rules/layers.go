// Layer application is CR 613: characteristics settle in a fixed layer order
// (copy, control, text, type, color, abilities, power/toughness), and within
// layer 7 in a further sublayer order (characteristic-defining, setting,
// modifying, counters, switching). M1 only produces effects in layers 6 and
// 7, but the full ladder is defined now so a later layer 2 control-change or
// layer 4 type-change effect is an addition to this file, not a rewrite of
// it — that retrofit is the project's own top-named risk.
package rules

import (
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// staticEffects reads every battlefield permanent's own S:Mode$ Continuous
// statics into ContinuousEffects, so a card whose entire rules text is a
// static (an Equipment's EquippedBy pump, an Aura's EnchantedBy pump, a
// vanilla lord's global pump) actually applies instead of silently doing
// nothing. Task 14 is what first wires static text into the layer system:
// until it, every S: line this build cared about was a play restriction
// (CantBeCast etc.), and the Mode$ Continuous statics every creature-lord
// and echo of Equipment/Enchant text carries were parsed but never turned
// into an effect -- a card with nothing but static text read as "does
// nothing".
//
// Reading these live off the battlefield permanent (the same scan activeStatics
// performs for restrictions) rather than registering them at ETB means a
// permanent placed on the battlefield by any path -- cast, a raw MoveZone in
// a test -- is covered, the effect expiry problem is solved for free (active()
// only walks battlefield permanents, so a departed source contributes nothing
// this call), and there is no registration event to keep in step with replay.
// Each static breaks into one effect per layer it touches -- AddPower/
// AddToughness is a layer-7 modify, AddKeyword a layer-6 grant, AddTypes a
// layer-4 type change -- so the layer ordering Derived applies (CR 613) still
// holds when one static carries both a pump and a keyword (exactly the Sword
// of Fire and Ice / Umezawa shape Task 14's tests build). The scan order is
// deterministic: AliveFrom(0) walks seats in fixed APNAP order, each
// battlefield zone is a slice, and each face's Statics is its parsed script
// order -- nothing here ranges a map, so the resulting option/view/settle
// order stays reproducible run to run (determinism requirement 3 of the
// dispatch).
func (e *Engine) staticEffects() []ContinuousEffect {
	var out []ContinuousEffect
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			o := e.G.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil {
				continue
			}
			for _, st := range f.Statics {
				if st.Mode != "Continuous" {
					continue
				}
				affects := st.Params["Affected"]
				if affects == "" {
					continue
				}
				base := ContinuousEffect{
					Source:     id,
					Timestamp:  o.Timestamp,
					Controller: o.Controller,
					Affects:    affects,
				}
				if hasStat(st, "AddPower") || hasStat(st, "AddToughness") {
					pt := base
					pt.Layer, pt.Sub = LPT, SubModify
					pt.AddPower = statInt(st, "AddPower")
					pt.AddToughness = statInt(st, "AddToughness")
					out = append(out, pt)
				}
				if hasStat(st, "AddKeyword") {
					kw := base
					kw.Layer = LAbilities
					kw.AddKeywords = statList(st, "AddKeyword")
					out = append(out, kw)
				}
				if hasStat(st, "AddType") || hasStat(st, "AddTypes") {
					ty := base
					ty.Layer = LType
					ty.AddTypes = statList(st, "AddTypes")
					if len(ty.AddTypes) == 0 {
						ty.AddTypes = statList(st, "AddType")
					}
					out = append(out, ty)
				}
			}
		}
	}
	return out
}

// hasStat reports whether a static line carries the named parameter.
func hasStat(st cards.Static, key string) bool {
	_, ok := st.Params[key]
	return ok
}

// statInt parses an S: line's numeric parameter, defaulting to 0 for a
// missing or unparseable value. Unlike parseAmount this tolerates negative
// values, because a static pump can lower power/toughness (-1) while
// parseAmount's clamp exists for the cost/characteristic modes that must
// never go negative.
func statInt(st cards.Static, key string) int32 {
	n, err := strconv.Atoi(strings.TrimSpace(st.Params[key]))
	if err != nil {
		return 0
	}
	return int32(n)
}

// statList splits a comma-separated additive parameter (AddKeyword,
// AddTypes) into its members.
func statList(st cards.Static, key string) []string {
	var out []string
	for _, v := range strings.Split(st.Params[key], ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Layer, Sublayer and ContinuousEffect moved to state/continuous.go in Task
// 19c, so effects primitives (which sit below rules and must never import
// it) can build a ContinuousEffect and hand it to this engine through
// effects.Host. These aliases and re-exported constants keep this package's
// own API -- and every existing caller and test in this package -- unchanged:
// only the canonical type definition moved, not its name or behaviour here.
type (
	Layer            = state.Layer
	Sublayer         = state.Sublayer
	ContinuousEffect = state.ContinuousEffect
)

const (
	LCopy      = state.LCopy
	LControl   = state.LControl
	LText      = state.LText
	LType      = state.LType
	LColor     = state.LColor
	LAbilities = state.LAbilities
	LPT        = state.LPT
)

const (
	SubNone     = state.SubNone
	SubCDA      = state.SubCDA
	SubSet      = state.SubSet
	SubModify   = state.SubModify
	SubCounters = state.SubCounters
	SubSwitch   = state.SubSwitch
)

// Derived is a permanent's current characteristics after every applicable
// continuous effect has been applied in CR 613 order. Nothing outside this
// file may read printed power, toughness or keywords directly — Derived (or
// the Power/Toughness/HasKeyword/Keywords accessors below) is the only path.
type Derived struct {
	Power, Toughness int32
	Keywords         []string
	Types            []string
}

// AddContinuous registers one continuous effect. A zero Timestamp is
// stamped from the game clock, so callers that do not care about relative
// ordering against other effects created in the same instant need not touch
// the clock themselves; the layer tests that DO care set Timestamp
// explicitly and bypass this.
//
// Ruling T19-a: the clock advances only through a logged ClockTick event,
// never a direct write to e.G.Clock. Object.Timestamp (see events.Move) is
// stamped from this same clock whenever a permanent enters the battlefield,
// so a direct write here would leave a game reconstructed from the log alone
// off by one on every later Timestamp — the same bug class Ruling T11-a
// already fixed for Passes/Priority.
func (e *Engine) AddContinuous(ce ContinuousEffect) {
	if ce.Timestamp == 0 {
		e.emit(events.Event{Kind: events.ClockTick})
		ce.Timestamp = e.G.Clock
	}
	// A Duration$ that spans the controller's NEXT turn (UntilYourNextTurn /
	// UntilTheEndOfYourNextTurn) gets a real turn-boundary lifetime: compute
	// the turn at whose end the effect expires from the live rotation, so it
	// outlives its source (a one-shot spell is already off the battlefield by
	// the time it registered) and is dropped by EndOfTurnCleanup when e.G.Turn
	// reaches UntilTurn -- not at the end of the current turn (UntilEOT) nor
	// never (source-leaves). Recomputing it here each re-execution is what
	// makes the boundary byte-identical on replay. If the controller cannot
	// be found in the alive rotation (eliminated) the effect gets no turn
	// boundary and falls back to the source-leaves rule.
	if effects.IsNextTurnDuration(ce.Duration) && ce.UntilTurn == 0 {
		ce.UntilTurn = e.nextTurnFor(ce.Controller)
	}
	e.continuous = append(e.continuous, ce)
	// Bump the cache version: active() (below) caches its sorted effect list
	// on (log head, continuousVersion), and this is the write that changes
	// e.continuous. The ClockTick above moved the log head too, but naming
	// the dependency explicitly here keeps active()'s invalidation correct
	// even if a future caller adds a continuous effect without an event.
	e.continuousVersion++
}

// nextTurnFor returns the turn number of the next turn (strictly after the
// current one) whose active player is p -- i.e. p's NEXT turn, the
// controller's-next-turn boundary of an UntilYourNextTurn effect.
// It walks the alive rotation from the current active player and returns 0
// (no turn boundary) if p is not alive, bounding the walk so an eliminated
// controller cannot spin the loop forever: at most AliveCount successors are
// distinct alive seats, so a full cycle without hitting p proves p is gone.
func (e *Engine) nextTurnFor(p state.PlayerID) int32 {
	alive := e.G.AliveCount()
	t := e.G.Turn
	q := e.G.Active
	for i := 0; i < alive; i++ {
		q = e.G.NextAlive(q)
		t++
		if q == p {
			return t
		}
	}
	return 0
}

// EndOfTurnCleanup drops every "until end of turn" effect (CR 514.2), and
// every turn-boundary effect whose expiry turn is the one now ending. Called
// from rules/combat.go's cleanupStep, which runs it on entry to the cleanup
// step.
func (e *Engine) EndOfTurnCleanup() {
	kept := e.continuous[:0]
	for _, ce := range e.continuous {
		if ce.UntilEOT {
			continue
		}
		if ce.UntilTurn != 0 && ce.UntilTurn == e.G.Turn {
			continue
		}
		kept = append(kept, ce)
	}
	e.continuous = kept
	// Bump the version for the same reason AddContinuous does: the cache is
	// keyed on continuousVersion, and this in-place rewrite (which emits no
	// event and moves no log head) drops every UntilEOT pump and every
	// expired UntilTurn effect. Without the bump, a stale active() cache
	// would keep reporting a dead pump's P/T.
	e.continuousVersion++
}

// active returns the effects that still exist, sorted into CR 613 order:
// layer, then sublayer, then timestamp. Ties within a (layer, sublayer,
// timestamp) triple — two effects created in the same AddContinuous batch
// without distinct timestamps — keep the order they were registered in,
// because sort.SliceStable never reorders equal elements; that registration
// order is itself deterministic (single goroutine, no map iteration), so the
// whole sort is reproducible run to run and safe for replay.
//
// Effects whose source has left the battlefield are dropped, which is what
// makes a lord's static bonus vanish the instant the lord dies. An effect
// marked UntilEOT is different: it is a one-shot pump that already resolved
// (Giant Growth), so it outlives its source and is only removed by
// EndOfTurnCleanup.
func (e *Engine) active() []ContinuousEffect {
	e.activeDepth++
	defer func() { e.activeDepth-- }()
	// Cached hit: derived only reads the returned slice, never mutates it, so
	// every Derived call of a board build shares this one sorted list. The
	// key is the log head plus the continuous-mutation version; a mismatch
	// means something the list depends on changed and the cache is stale.
	if e.activeEpoch == len(e.L.Events) && e.activeVersion == e.continuousVersion {
		return e.activeBuf
	}
	e.activeEpoch = len(e.L.Events)
	e.activeVersion = e.continuousVersion
	buf := e.activeBuf[:0]
	if e.activeDepth > 1 {
		// Re-entrant (a nested Derived mid-rebuild): own a private list rather
		// than overwrite the outer call's result mid-range. Same guard Task A2
		// uses for forEachObject. (This path is effectively unreachable — a
		// Derived call never emits an event, so the epoch cannot move mid-
		// range — but it keeps the buffer discipline airtight.
		buf = nil
	}
	for _, ce := range e.continuous {
		if ce.UntilEOT {
			buf = append(buf, ce)
			continue
		}
		if ce.UntilTurn != 0 {
			// A turn-boundary effect outlives its source (a one-shot spell is
			// already gone) and is active through its own expiry turn, dropped
			// only by EndOfTurnCleanup when e.G.Turn reaches UntilTurn. Keep it
			// while the current turn is at or before that boundary; the
			// `<=` is the guard that keeps an effect from lingering if a
			// cleanup were ever skipped.
			if e.G.Turn <= ce.UntilTurn {
				buf = append(buf, ce)
			}
			continue
		}
		if o := e.G.Obj(ce.Source); o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		buf = append(buf, ce)
	}
	// The static-derived effects come from the memoized scan (see
	// Engine.staticContinuous): refreshed once per emitted event, not per
	// Derived call, so a board-wide scan does not dominate the hottest path.
	// A version-only rebuild (EndOfTurnCleanup dropping an UntilEOT pump) is a
	// subset of the rebuild condition that leaves the battlefield permanent
	// set, and therefore the static memo, untouched — so the two are checked
	// independently exactly as before.
	if e.staticEpoch != len(e.L.Events) {
		e.staticEpoch = len(e.L.Events)
		e.staticContinuous = e.staticEffects()
	}
	buf = append(buf, e.staticContinuous...)
	sort.SliceStable(buf, func(i, j int) bool {
		if buf[i].Layer != buf[j].Layer {
			return buf[i].Layer < buf[j].Layer
		}
		if buf[i].Sub != buf[j].Sub {
			return buf[i].Sub < buf[j].Sub
		}
		return buf[i].Timestamp < buf[j].Timestamp
	})
	if e.activeDepth <= 1 {
		// Keep the grown, sorted buffer on the Engine for the next build or
		// cache hit; a re-entrant build's private buffer is discarded on return.
		e.activeBuf = buf
	}
	return buf
}

// derivedScalar returns only an object's derived power and toughness — the
// subset of Derived that combat, legal, cast, trigger and bot predicates read
// constantly (and, through the Chars interface, every projected view). It
// never builds the keyword/type slices Derived carries, and its effect list
// comes from active()'s cached, buffer-reused build, so it allocates nothing
// per call. Its P/T is identical to what the full Derived computes because an
// LPT effect's applicability never depends on the derived keyword/type
// grants: MatchesSpecFrom (effects/filter.go) resolves every predicate
// against the object's *printed* face (o.Face().HasKeyword / o.Face().Types),
// never against the grants Derived accumulates. So no layer-6/4 grant can
// flip a layer-7 pump, and a pass that reads only LPT effects reproduces the
// same power and toughness Derived did before — skipping the other layers is
// a saving, never a behaviour change.
func (e *Engine) derivedScalar(id state.ObjID) (power, toughness int32) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return 0, 0
	}
	f := o.Face()
	power, toughness = int32(f.Power()), int32(f.Toughness())
	for _, ce := range e.active() {
		if ce.Layer != LPT {
			continue
		}
		if !effects.MatchesSpecFrom(e.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		switch ce.Sub {
		case SubSet:
			if ce.HasSet {
				power, toughness = ce.SetPower, ce.SetToughness
			}
		case SubModify:
			power += ce.AddPower
			toughness += ce.AddToughness
		}
	}
	// 7d: counters apply after every other layer-7 effect (CR 613.4).
	if n := o.Counter("P1P1"); n != 0 {
		power += n
		toughness += n
	}
	if n := o.Counter("M1M1"); n != 0 {
		power -= n
		toughness -= n
	}
	return power, toughness
}

// Derived computes an object's current characteristics: printed values from
// its face, then every applicable continuous effect in layer order, then
// layer 7d counters last. A malformed or missing object degrades to the
// zero Derived rather than panicking — layer inputs ultimately come from
// parsed card text, and a nonexistent ObjID or an ability/token object with
// no Face() must never crash the match goroutine.
//
// The Keywords and Types slices alias the Engine's scratch buffers
// (derivedKW / derivedTypes, engine.go) and are reused across calls: after
// the first call's buffers grow to size they are rewritten, never
// reallocated, so repeated Derived builds are allocation-free. That is only
// sound because every caller treats the returned slices as read-only and
// does not retain them past building its own view — view.Project and
// botpolicy both copy (append([]string(nil), ...)) synchronously, and the
// loops in HasKeyword and protectedFrom only range. Sharing would be wrong
// if a caller held one Derived's slices while calling Derived again (the
// next call would rewrite the shared buffers), so the discipline is
// load-bearing; derivedDepth guards re-entry the way active()'s activeDepth
// guards its cache (a nested Derived mid-build gets private owned buffers
// instead of clobbering the outer build's).
func (e *Engine) Derived(id state.ObjID) Derived {
	power, toughness := e.derivedScalar(id)
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Derived{Power: power, Toughness: toughness}
	}
	f := o.Face()
	e.derivedDepth++
	kw := e.derivedKW
	ty := e.derivedTypes
	if e.derivedDepth > 1 {
		// Re-entrant (a nested Derived mid-build): own private buffers rather
		// than overwrite the outer call's backing arrays mid-range. Same guard
		// Task A2 uses for forEachObject and this file uses for active(). (This
		// path is effectively unreachable — MatchesSpecFrom reads faces, never
		// calls Derived — but it keeps the buffer discipline airtight.)
		kw = nil
		ty = nil
	}
	kw = append(kw[:0], f.Keywords...)
	ty = append(ty[:0], f.Types...)
	for _, ce := range e.active() {
		if !effects.MatchesSpecFrom(e.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		switch ce.Layer {
		case LAbilities:
			kw = append(kw, ce.AddKeywords...)
		case LType:
			ty = append(ty, ce.AddTypes...)
		}
	}
	if e.derivedDepth <= 1 {
		// Keep the grown buffers on the Engine for the next build; a re-entrant
		// build's private buffers are discarded on return.
		e.derivedKW = kw
		e.derivedTypes = ty
	}
	e.derivedDepth--
	return Derived{Power: power, Toughness: toughness, Keywords: kw, Types: ty}
}

func (e *Engine) Power(id state.ObjID) int32 {
	p, _ := e.derivedScalar(id)
	return p
}
func (e *Engine) Toughness(id state.ObjID) int32 {
	_, t := e.derivedScalar(id)
	return t
}

// HasKeyword matches case-insensitively, like its sibling cards.Face.HasKeyword
// (Ruling T19-b) — every existing call site already goes through that
// case-insensitive comparison, so an exact-match Engine.HasKeyword would have
// been a silent trap for the first caller with non-canonical-cased input.
func (e *Engine) HasKeyword(id state.ObjID, kw string) bool {
	for _, k := range e.Derived(id).Keywords {
		if strings.EqualFold(cardsKeywordHead(k), kw) {
			return true
		}
	}
	return false
}

// RegenerationDisallowed implements effects.Host for the CantRegenerate
// restriction (Task ce1): reports whether an Effect-registered restriction
// forbids id from regenerating. Consulted by effects.ReplaceDestruction, so
// Incinerate's "creature can't be regenerated this turn" actually blocks the
// shield-consumption path instead of being a Note. Scanning e.active() keeps
// the expiry discipline identical to every other continuous effect: a
// this-turn restriction is UntilEOT and is dropped at cleanup, a permanent-
// sourced one disappears when its source leaves the battlefield.
func (e *Engine) RegenerationDisallowed(id state.ObjID) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantRegenerate" {
			continue
		}
		if e.restrictionApplies(ce, id) {
			return true
		}
	}
	return false
}

// restrictionBlocksTarget reports whether an Effect-registered CantTarget
// restriction (Vines of Vastwood) prevents the player actor from targeting id
// with a spell or ability. Called from rules/stack.go's askTarget alongside
// the protectedFrom check, so a creature granted "can't be the target of
// spells or abilities your opponents control this turn" is actually withheld
// from the opponent's targeting options.
func (e *Engine) restrictionBlocksTarget(id state.ObjID, actor state.PlayerID) bool {
	for _, ce := range e.active() {
		if ce.Restriction != "CantTarget" {
			continue
		}
		if !e.restrictionApplies(ce, id) {
			continue
		}
		if !e.restrictionActorMatches(ce, actor) {
			continue
		}
		return true
	}
	return false
}

// restrictionApplies reports whether a registered restriction's ValidCard$/
// ValidTarget$ spec selects the object id. The Forge filter grammar has no
// IsRemembered predicate, so the remembered-object set the Effect captured is
// matched directly (the dominant shape for Vines/Incinerate); any other spec
// falls back to the ordinary spec matcher so a restriction that names a
// quality (CantTarget with ValidCard$ Creature, say) still works.
func (e *Engine) restrictionApplies(ce ContinuousEffect, id state.ObjID) bool {
	spec := ce.RestrictParams["ValidCard"]
	if spec == "" {
		spec = ce.RestrictParams["ValidTarget"]
	}
	if spec != "" && strings.Contains(spec, "IsRemembered") {
		// A COMPOUND IsRemembered spec (Card.IsRemembered+Creature, a + AND or
		// a , OR list) cannot be resolved by the remembered-set match alone:
		// the extra predicate would be silently dropped and the restriction
		// would over-apply to a remembered object that fails it. No corpus
		// restriction static carries one (measured; see AGENTS.md / report), so
		// reject it here -- the restriction does not apply, the same
		// "unsupported" fallback the Note path uses -- rather than mis-apply.
		if strings.ContainsAny(spec, "+,") {
			return false
		}
		for _, r := range ce.Remembered {
			if r == id {
				return true
			}
		}
		return false
	}
	if spec == "" {
		return len(ce.Remembered) > 0
	}
	return effects.MatchesSpecFrom(e.G, spec, id, ce.Controller, ce.Source)
}

// restrictionActorMatches scopes a CantTarget restriction by Activator$:
// Vines of Vastwood's Activator$ Player.Opponent means the restriction only
// bites when the player targeting the creature is an opponent of the effect's
// controller (the caster of Vines). A restriction with no Activator$ applies
// to any actor.
func (e *Engine) restrictionActorMatches(ce ContinuousEffect, actor state.PlayerID) bool {
	spec, ok := ce.RestrictParams["Activator"]
	if !ok {
		return true
	}
	return effects.MatchesPlayerSpec(e.G, spec, actor, ce.Controller)
}

// Keywords exists for Ruling F2: Task 23's view.Chars interface needs a
// Keywords(state.ObjID) []string method, and Engine.Derived already returns
// a Derived struct — a method of the same name on Engine could not satisfy
// an interface expecting a slice. This is that method; Derived(id).Keywords
// remains the field other engine-internal code should read when it also
// wants Power/Toughness/Types in the same call.
func (e *Engine) Keywords(id state.ObjID) []string { return e.Derived(id).Keywords }
