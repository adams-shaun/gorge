package rules

import (
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
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
// 115.5). An attach SA (API$ Attach, the Equip/Reconfigure expansion) never
// offers its own source permanent, the exclusion targetAsk applies at
// activation: a copy has the original's source (CR 707.10b) and may choose
// only what the original could (CR 707.10c), and an Equipment can't equip
// itself (CR 301.5c).
func (e *Engine) LegalRetargets(obj state.ObjID, sa *cards.SA, parent []state.Target) []state.Target {
	o := e.G.Obj(obj)
	if o == nil {
		return nil
	}
	savedResolving := e.resolvingObj
	e.resolvingObj = 0
	defer func() { e.resolvingObj = savedResolving }()
	var out []state.Target
	if parent != nil {
		out = e.LegalSubTargets(o.Controller, obj, sa, parent)
	} else {
		out = e.LegalTargets(o.Controller, obj, sa)
	}
	if src := attachSelfSource(e.G, o, sa); src != 0 {
		out = slices.DeleteFunc(out, func(t state.Target) bool { return !t.IsPlayer && t.Obj == src })
	}
	return out
}

// attachSelfDestination reports whether an attach SA (API$ Attach) fastens
// its own source: Object$ absent or Self, the Equip/Reconfigure expansion's
// shape. Only then is the source permanent an impossible destination (CR
// 301.5c: an Equipment can't equip itself; CR 701.3a). An Object$ naming
// another permanent (ParentTarget, Remembered, Valid, ...) attaches THAT
// object instead, so the ability's source stays a legal destination -- Brass
// Squire may attach an Equipment to itself. This is the one home for the
// exclusion the offer census, the chain availability census and targetAsk all
// read.
func attachSelfDestination(sa *cards.SA) bool {
	if sa == nil || sa.API != "Attach" {
		return false
	}
	if ap := effects.AttachOf(sa); ap.ObjectPresent && ap.Object != "Self" {
		return false
	}
	return true
}

// attachSelfSource is the permanent an attach SA (API$ Attach) on stack
// object o may never target: o's source, the exclusion targetAsk applies at
// activation (CR 301.5c, 701.3a). A new-target census of o (a redirect, or a
// copy's CR 707.10c election -- the copy keeps the original's source, CR
// 707.10b) drops it so it offers only what the original could target. Zero
// when sa is no attach SA, its destination is not its own source, or o has no
// source. Also zero once the source left the battlefield and came back (CR
// 400.7): the permanent there is a new object, "another target creature" can
// name it, and the ability's old source attaches to nothing (the dirac
// kernel offers it the same way).
func attachSelfSource(g *state.Game, o *state.Object, sa *cards.SA) state.ObjID {
	if !attachSelfDestination(sa) || effects.SourceIsNewObject(g, o) {
		return 0
	}
	return o.Source
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
		e.removeSubTarget(obj, old)
	}
	e.recordCastSub(obj, link.Line, ts)
	e.recordSubTargets(obj, kept)
}

// removeSubTarget takes one entry out of obj's flat Object.SubTargets.
func (e *Engine) removeSubTarget(obj state.ObjID, t state.Target) {
	ev := events.Event{Kind: events.SubTargetRemoved, Obj: obj, Amount: 2}
	if t.IsPlayer {
		ev.Amount, ev.Player = 3, t.Player
	} else {
		ev.IDs = []state.ObjID{t.Obj}
	}
	e.emit(ev)
}
