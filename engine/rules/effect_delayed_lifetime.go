package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// sweepEffectDelayed mirrors the continuous Effect rider sweeps. All edits to
// Game.Delayed are folded by Apply, never made in place. Snapshot IDs before
// emitting removals because emit itself can inspect the delayed registry.
func (e *Engine) sweepEffectDelayed(ev events.Event) {
	type change struct {
		id     uint32
		forget bool
	}
	var changes []change
	for _, dt := range e.G.Delayed {
		has := false
		for _, target := range dt.Remembered {
			if target.Obj == ev.Obj {
				has = true
				break
			}
		}
		if ev.Kind == events.MoveZone || ev.Kind == events.PutOnStack || ev.Kind == events.Draw {
			if !has {
				continue
			}
			if dt.ExileOnMoved != "" && effects.ParseZone(dt.ExileOnMoved) == ev.From {
				changes = append(changes, change{id: dt.ID})
				continue
			}
			if dt.ForgetOnMoved != "" && effects.ParseZone(dt.ForgetOnMoved) == ev.From {
				changes = append(changes, change{id: dt.ID, forget: true})
			}
		} else if ev.Kind == events.PhaseOut && ev.Amount < 0 && has && dt.ForgetOnPhasedIn {
			// ForgetOnPhasedIn$ (CR 702.25): the permanent is back, so the
			// Effect's memory of it ends; the last one retires the
			// registration, whose comeback body would otherwise toggle the
			// returned permanent out on the host's next untap or departure.
			if len(dt.Remembered) == 1 {
				changes = append(changes, change{id: dt.ID})
			} else {
				changes = append(changes, change{id: dt.ID, forget: true})
			}
		} else if ev.Kind == events.CounterChange && ev.Amount < 0 && has && dt.ForgetCounter == ev.Counter {
			if obj := e.G.Obj(ev.Obj); obj == nil || obj.Counter(ev.Counter) == 0 {
				changes = append(changes, change{id: dt.ID, forget: true})
			}
		}
	}
	for _, change := range changes {
		kind := events.DelayedRemove
		if change.forget {
			kind = events.DelayedForget
		}
		e.emit(events.Event{Kind: kind, Amount: int32(change.id), Obj: ev.Obj})
	}
}

// A cast is only complete when the deferred trigger walk begins, after cost
// payment. Never consume a grant on a proposal that CR 733.1 reverses.
func (e *Engine) sweepEffectDelayedCast(ev events.Event) {
	var ids []uint32
	for _, dt := range e.G.Delayed {
		spec := strings.TrimSpace(dt.ForgetOnCast)
		if spec == "" {
			continue
		}
		sc := e.specCtx(dt.Source, dt.Controller)
		sc.Remembered = append(sc.Remembered, dt.Remembered...)
		if e.matchesSpec(spec, ev.Obj, sc) {
			ids = append(ids, dt.ID)
		}
	}
	for _, id := range ids {
		e.emit(events.Event{Kind: events.DelayedRemove, Amount: int32(id)})
	}
}

// The host's effect-token exile ends every imprinted registration of that
// source, including delayed triggers (not just continuous grants).
func (e *Engine) endImprintedDelayed(source state.ObjID) {
	var ids []uint32
	for _, dt := range e.G.Delayed {
		if dt.ImprintOnHost && dt.Source == source {
			ids = append(ids, dt.ID)
		}
	}
	for _, id := range ids {
		e.emit(events.Event{Kind: events.DelayedRemove, Amount: int32(id)})
	}
}

// sweepControlAndPhase runs the Effect-lifetime sweeps that ride a control
// change or a phase transition: AsLongAsControl$ expiry on a control change or
// a phase-out, and the ForgetOnPhasedIn$ sweep on a phase-in (Amount < 0).
// Kept out of emit so its branch count does not grow.
func (e *Engine) sweepControlAndPhase(ev events.Event) {
	if ev.Kind == events.ControlChange || ev.Kind == events.PhaseOut && ev.Amount >= 1 {
		e.expireAsLongAsControl(ev.Obj)
	}
	if ev.Kind == events.PhaseOut && ev.Amount < 0 {
		e.sweepEffectDelayed(ev)
	}
}
