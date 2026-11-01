package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// LegalSubTargets implements effects.Host: the target census for a
// SubAbility$'s own ValidTgts$ asked while its parent resolves, with the
// parent's already-chosen targets bound for the Targeted*/ParentTarget
// referents (effects.SpecContext.ParentTargets). The binding is engine
// scratch set and restored inside this one synchronous census.
func (e *Engine) LegalSubTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA, parent []state.Target) []state.Target {
	saved, savedBound := e.subOfferParent, e.subOfferBound
	e.subOfferParent, e.subOfferBound = parent, true
	defer func() { e.subOfferParent, e.subOfferBound = saved, savedBound }()
	return e.LegalTargets(chooser, source, sa)
}

// LegalRetargets implements effects.Host. The census runs as a target offer
// of obj itself, so the resolving redirect's identity stands aside: obj is the
// source protection judges and the object that cannot target itself (CR
// 115.5).
func (e *Engine) LegalRetargets(obj state.ObjID, sa *cards.SA, parent []state.Target) []state.Target {
	o := e.G.Obj(obj)
	if o == nil {
		return nil
	}
	savedResolving := e.resolvingObj
	e.resolvingObj = 0
	defer func() { e.resolvingObj = savedResolving }()
	if parent != nil {
		return e.LegalSubTargets(o.Controller, obj, sa, parent)
	}
	return e.LegalTargets(o.Controller, obj, sa)
}

// AnnouncedLinkTargets implements effects.Host.
func (e *Engine) AnnouncedLinkTargets(obj state.ObjID, link *cards.SA) []state.Target {
	return e.castSubTargets[obj][link.Line]
}

// RetargetLink implements effects.Host. Object.SubTargets is the object's flat
// target list, so it changes by what left and what arrived, and the per-link
// record that resolution and the CR 608.2b recheck read takes the new list
// whole.
func (e *Engine) RetargetLink(obj state.ObjID, link *cards.SA, ts []state.Target) {
	kept := slices.Clone(ts)
	for _, old := range e.castSubTargets[obj][link.Line] {
		if i := slices.Index(kept, old); i >= 0 {
			kept = slices.Delete(kept, i, i+1)
			continue
		}
		ev := events.Event{Kind: events.SubTargetRemoved, Obj: obj, Amount: 2}
		if old.IsPlayer {
			ev.Amount, ev.Player = 3, old.Player
		} else {
			ev.IDs = []state.ObjID{old.Obj}
		}
		e.emit(ev)
	}
	e.recordCastSub(obj, link.Line, ts)
	e.recordSubTargets(obj, kept)
}
