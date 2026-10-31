package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// followTriggerObject switches the triggers ev just queued from the snapshot
// taken before ev to a live read of ev's object. Only a departure looks back
// in time (CR 603.10): after an entry, a cast or a control change the object
// is still there, so "its power" is its current power (CR 608.2h).
func (e *Engine) followTriggerObject(ev *events.Event, queued int) {
	if ev.Obj == 0 || !(entersBattlefield(ev) || ev.Kind == events.PutOnStack || ev.Kind == events.ControlChange) {
		return
	}
	for i := queued; i < len(e.pendingTriggers); i++ {
		pt := &e.pendingTriggers[i]
		if pt.Ctx.LKI != nil && pt.Ctx.LKI.ID != ev.Obj {
			continue
		}
		pt.Ctx.LKI = nil
		pt.Ctx.Snap.Power, pt.Ctx.Snap.Toughness, pt.Ctx.Snap.PTValid = 0, 0, false
		pt.follows = ev.Obj
	}
}

// settleFollowedObject gives every trigger still reading lki's object live
// the snapshot of it taken just before it moves.
func (e *Engine) settleFollowedObject(lki *state.Object, power, toughness int32, ptValid bool) {
	for i := range e.pendingTriggers {
		pt := &e.pendingTriggers[i]
		if pt.follows != lki.ID {
			continue
		}
		cp := lki.CloneDeep()
		pt.Ctx.LKI = &cp
		pt.Ctx.Snap.Power, pt.Ctx.Snap.Toughness, pt.Ctx.Snap.PTValid = power, toughness, ptValid
		pt.follows = 0
	}
	for id, held := range e.triggerLKI {
		if held.follows != lki.ID {
			continue
		}
		cp := lki.CloneDeep()
		e.triggerLKI[id] = triggerObjectLKI{object: &cp, power: power, toughness: toughness, ptValid: ptValid}
	}
}

// keepTriggerLKI records what the stack object id knows about its causing
// event's object, for resolution and target checks to read.
func (e *Engine) keepTriggerLKI(id state.ObjID, pt *pendingTrigger) {
	var held triggerObjectLKI
	switch {
	case pt.Ctx.LKI != nil:
		lki := pt.Ctx.LKI.CloneDeep()
		held = triggerObjectLKI{object: &lki,
			power: pt.Ctx.Snap.Power, toughness: pt.Ctx.Snap.Toughness,
			ptValid: pt.Ctx.Snap.PTValid}
	case pt.follows != 0:
		held.follows = pt.follows
	default:
		return
	}
	if e.triggerLKI == nil {
		e.triggerLKI = make(map[state.ObjID]triggerObjectLKI)
	}
	e.triggerLKI[id] = held
}
