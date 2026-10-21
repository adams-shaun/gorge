package rules

import (
	"github.com/adams-shaun/gorge/engine/state"
)

// fireTimeRememberedStamps stamps every remembered OBJECT with its
// Incarnation as the trigger fired -- the state the synchronous emit check
// sees, right after the triggering event applied. It rides the queued
// trigger's effects.TriggerContext.TriggerRememberedStamps so a body that
// resolves later (a dies trigger waiting on the stack through a round trip)
// stamps a delayed registration with the incarnation the triggering event
// left, not the one the object carries at resolution: the trigger refers to
// the object that existed when it fired (CR 400.7). Player references and
// objects that no longer resolve carry no stamp, the same shape
// events.rememberedStampsFrom keeps.
func (e *Engine) fireTimeRememberedStamps(remembered []state.Target) []state.TargetStamp {
	var out []state.TargetStamp
	for _, t := range remembered {
		if t.IsPlayer {
			continue
		}
		if o := e.G.Obj(t.Obj); o != nil {
			out = append(out, state.TargetStamp{Obj: t.Obj, Incarnation: o.Incarnation, ZoneGen: o.ZoneGen})
		}
	}
	return out
}

// rememberedMovedSinceFire lists, in stamp order, the stamped objects whose
// ZoneGen has moved since the trigger fired: each is a new object the
// trigger's referents cannot find (CR 400.7). It is computed once as the
// trigger starts to resolve, so moves the resolution itself makes (CR
// 400.7j: "exile it, then return it") are never listed. nil when none.
func (e *Engine) rememberedMovedSinceFire(stamps []state.TargetStamp) []state.ObjID {
	var out []state.ObjID
	for _, s := range stamps {
		if o := e.G.Obj(s.Obj); o != nil && o.ZoneGen != s.ZoneGen {
			out = append(out, s.Obj)
		}
	}
	return out
}
