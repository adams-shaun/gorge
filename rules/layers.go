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
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

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
	out := make([]ContinuousEffect, 0, len(e.continuous))
	for _, ce := range e.continuous {
		if ce.UntilEOT {
			out = append(out, ce)
			continue
		}
		if o := e.G.Obj(ce.Source); o == nil || o.Zone != state.ZBattlefield {
			continue
		}
		out = append(out, ce)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Layer != out[j].Layer {
			return out[i].Layer < out[j].Layer
		}
		if out[i].Sub != out[j].Sub {
			return out[i].Sub < out[j].Sub
		}
		return out[i].Timestamp < out[j].Timestamp
	})
	return out
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
