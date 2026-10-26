package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// stat:CantPhaseIn / stat:CantPhaseOut (CR 702.26, task cr-g159) -- "it can't
// phase out" (Ertai's Familiar, Spatial Binding) and "those permanents can't
// phase in" (The Phasing of Zhalfir; Teferi, Timeless Voyager; Disciple of
// Caelus Nin's printed line). Forge's StaticAbilityCantPhase reads every
// static of the mode by ValidCard$ alone, consulted by the untap-step
// phasing (Untap.java) and PhasesEffect.
//
// Every phase change is one events.PhaseOut (Amount +1 out, -1 in), from the
// untap-step scan (finishUntapStep) and effPhases alike, so the restriction
// is consulted once at the engine's emit choke point (the cantBeSuspected
// shape): a blocked phasing is never logged, never folded (no attachment
// moves with it) and fires no phases-out trigger (Ertai's Familiar's mill).
//
// phaseBlocked takes the narrow cantPhaseEngine interface rather than
// *Engine so it adds no Engine method.

// cantPhaseEngine is the slice of Engine the prohibition read needs.
type cantPhaseEngine interface {
	Game() *state.Game
	active() []ContinuousEffect
	restrictionApplies(ce *ContinuousEffect, id state.ObjID) bool
	activeStatics(mode string) []staticView
	staticGateHolds(sv staticView) bool
	matchesSpec(spec string, id state.ObjID, sc effects.SpecContext) bool
	staticSpecCtx(sv staticView) effects.SpecContext
}

// suppressPhaseEvent reports whether ev is a phase change a live CantPhaseIn
// / CantPhaseOut forbids. Only a real change is consulted: a phase-out of a
// phased-in battlefield permanent, a phase-in of a phased-out one.
func suppressPhaseEvent(e cantPhaseEngine, ev events.Event) bool {
	if ev.Kind != events.PhaseOut || ev.Amount == 0 {
		return false
	}
	o := e.Game().Obj(ev.Obj)
	if o == nil || o.Zone != state.ZBattlefield {
		return false
	}
	in := ev.Amount < 0
	if in != o.PhasedOut {
		return false
	}
	return phaseBlocked(e, ev.Obj, in)
}

// phaseBlocked reports whether id can't phase in (in) or out (!in): an
// Effect-registered restriction of the mode whose ValidCard$ selects id
// (matched against the registration's remembered set), or a face static of
// the mode whose gate holds and whose ValidCard$ (absent: a blanket
// prohibition, the restrictive direction a "can't" takes) selects id.
func phaseBlocked(e cantPhaseEngine, id state.ObjID, in bool) bool {
	mode, statics := effects.ModeCantPhaseOut, e.activeStatics("CantPhaseOut")
	if in {
		mode, statics = effects.ModeCantPhaseIn, e.activeStatics("CantPhaseIn")
	}
	for ceI, ceL := 0, e.active(); ceI < len(ceL); ceI++ {
		ce := &ceL[ceI]
		if ce.Restriction == mode && e.restrictionApplies(ce, id) {
			return true
		}
	}
	for _, sv := range statics {
		if !e.staticGateHolds(sv) {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidCard); spec != "" &&
			!e.matchesSpec(spec, id, e.staticSpecCtx(sv)) {
			continue
		}
		return true
	}
	return false
}
