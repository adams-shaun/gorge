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
	e.continuous = append(e.continuous, ce)
	// Bump the cache version: active() (below) caches its sorted effect list
	// on (log head, continuousVersion), and this is the write that changes
	// e.continuous. The ClockTick above moved the log head too, but naming
	// the dependency explicitly here keeps active()'s invalidation correct
	// even if a future caller adds a continuous effect without an event.
	e.continuousVersion++
}

// EndOfTurnCleanup drops every "until end of turn" effect (CR 514.2).
// Called from rules/combat.go's cleanupStep, which runs it on entry to the
// cleanup step.
func (e *Engine) EndOfTurnCleanup() {
	kept := e.continuous[:0]
	for _, ce := range e.continuous {
		if !ce.UntilEOT {
			kept = append(kept, ce)
		}
	}
	e.continuous = kept
	// Bump the version for the same reason AddContinuous does: the cache is
	// keyed on continuousVersion, and this in-place rewrite (which emits no
	// event and moves no log head) drops every UntilEOT pump. Without the
	// bump, a stale active() cache would keep reporting a dead pump's P/T.
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

// Derived computes an object's current characteristics: printed values from
// its face, then every applicable continuous effect in layer order, then
// layer 7d counters last. A malformed or missing object degrades to the
// zero Derived rather than panicking — layer inputs ultimately come from
// parsed card text, and a nonexistent ObjID or an ability/token object with
// no Face() must never crash the match goroutine.
func (e *Engine) Derived(id state.ObjID) Derived {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return Derived{}
	}
	f := o.Face()
	d := Derived{
		Power:     int32(f.Power()),
		Toughness: int32(f.Toughness()),
		Types:     append([]string(nil), f.Types...),
	}
	d.Keywords = append(d.Keywords, f.Keywords...)

	for _, ce := range e.active() {
		if !effects.MatchesSpecFrom(e.G, ce.Affects, id, ce.Controller, ce.Source) {
			continue
		}
		switch ce.Layer {
		case LAbilities:
			d.Keywords = append(d.Keywords, ce.AddKeywords...)
		case LType:
			d.Types = append(d.Types, ce.AddTypes...)
		case LPT:
			switch ce.Sub {
			case SubSet:
				if ce.HasSet {
					d.Power, d.Toughness = ce.SetPower, ce.SetToughness
				}
			case SubModify:
				d.Power += ce.AddPower
				d.Toughness += ce.AddToughness
			}
		}
	}
	// 7d: counters apply after every other layer-7 effect (CR 613.4).
	if n := o.Counter("P1P1"); n != 0 {
		d.Power += n
		d.Toughness += n
	}
	if n := o.Counter("M1M1"); n != 0 {
		d.Power -= n
		d.Toughness -= n
	}
	return d
}

func (e *Engine) Power(id state.ObjID) int32     { return e.Derived(id).Power }
func (e *Engine) Toughness(id state.ObjID) int32 { return e.Derived(id).Toughness }

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

// Keywords exists for Ruling F2: Task 23's view.Chars interface needs a
// Keywords(state.ObjID) []string method, and Engine.Derived already returns
// a Derived struct — a method of the same name on Engine could not satisfy
// an interface expecting a slice. This is that method; Derived(id).Keywords
// remains the field other engine-internal code should read when it also
// wants Power/Toughness/Types in the same call.
func (e *Engine) Keywords(id state.ObjID) []string { return e.Derived(id).Keywords }
