package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// bindDelayedSourceDamageLKI carries a delayed-shape trigger's damage-source
// last-known information onto its stack object id, as the TriggerPush branch
// does for a face trigger. A Saga's chapter ability (checkChapterTriggers)
// queues in this shape, and its source can leave while it waits -- a second
// Phoenix, Warden of Fire put into the graveyard by the legend rule with
// chapter I still pending. captureSourceLifelinkLKI then snapshotted the
// pending trigger's Ctx.Snap, but without this the snapshot never reached the
// maps resolveTop reads, so the damage read the graveyard card (the front
// face, without lifelink) instead of the Phoenix as it last existed
// (CR 113.7a, 608.2h, 702.15b). An ability whose source never left has no
// snapshot and is untouched.
func (e *Engine) bindDelayedSourceDamageLKI(id state.ObjID, pt *pendingTrigger) {
	if pt.Ctx.Snap.SourceLifelinkValid {
		if e.sourceLifelinkLKI == nil {
			e.sourceLifelinkLKI = make(map[state.ObjID]bool)
		}
		e.sourceLifelinkLKI[id] = pt.Ctx.Snap.SourceLifelink
	}
	if pt.Ctx.Snap.SourceControllerValid {
		if e.sourceControllerLKI == nil {
			e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
		}
		e.sourceControllerLKI[id] = pt.Ctx.Snap.SourceController
	}
	if pt.Ctx.Snap.DamageSource != nil {
		if e.damageSourceLKI == nil {
			e.damageSourceLKI = make(map[state.ObjID]map[state.ObjID]effects.DamageSourceLKI)
		}
		e.damageSourceLKI[id] = cloneDamageSourceLKI(pt.Ctx.Snap.DamageSource)
	}
}
