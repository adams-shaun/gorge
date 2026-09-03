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

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// Layer is CR 613's application order.
type Layer uint8

const (
	LCopy      Layer = 1
	LControl   Layer = 2
	LText      Layer = 3
	LType      Layer = 4
	LColor     Layer = 5
	LAbilities Layer = 6
	LPT        Layer = 7
)

// Sublayer is CR 613.4's breakdown of layer 7.
type Sublayer uint8

const (
	SubNone     Sublayer = 0
	SubCDA      Sublayer = 1 // 7a characteristic-defining
	SubSet      Sublayer = 2 // 7b setting
	SubModify   Sublayer = 3 // 7c modifying
	SubCounters Sublayer = 4 // 7d counters
	SubSwitch   Sublayer = 5 // 7e switching
)

// ContinuousEffect is one active modification. Affects is a Forge filter spec
// evaluated with effects.MatchesSpecFrom against each object on the
// battlefield, so continuous effects reuse the same filter language as
// everything else rather than reimplementing predicate matching.
type ContinuousEffect struct {
	Source     state.ObjID
	Timestamp  uint32
	Layer      Layer
	Sub        Sublayer
	Affects    string
	Controller state.PlayerID
	UntilEOT   bool

	AddPower, AddToughness int32
	SetPower, SetToughness int32
	HasSet                 bool
	AddKeywords            []string
	AddTypes               []string
}

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
func (e *Engine) AddContinuous(ce ContinuousEffect) {
	if ce.Timestamp == 0 {
		e.G.Clock++
		ce.Timestamp = e.G.Clock
	}
	e.continuous = append(e.continuous, ce)
}

// EndOfTurnCleanup drops every "until end of turn" effect. This is called
// from the cleanup step (CR 514.2) — by Task 21, which owns turn structure.
// Nothing in this package calls it; it is defined and tested here in
// isolation so Task 21 has a working primitive to wire in.
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

func (e *Engine) HasKeyword(id state.ObjID, kw string) bool {
	for _, k := range e.Derived(id).Keywords {
		if cardsKeywordHead(k) == kw {
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
