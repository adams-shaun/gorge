package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// unattachedMatches implements Mode$ Unattached (CR 701.3b), the
// "whenever CARDNAME becomes unattached from a permanent" trigger the four
// corpus carriers print (Captain's Hook, Grafted Exoskeleton, Grafted
// Wargear, Stitcher's Graft -- all Equipment). The four lines share one
// shape: `ValidAttachment$ Card.Self | ValidObject$ Permanent`.
//
// Two events carry a detach. The first is events.Unattached: a distinct Kind
// is what lets this mode fire while Mode$ Attached keeps ignoring a detach,
// and the event carries the attachment in Obj and the FORMER BEARER in
// IDs[0]. engine/rules/attach.go's attachmentSBAs is the one emit site for
// the arms where the attachment stays on the battlefield -- including the
// bearer-left detach, which the CR 400.7 departure fold in events' Move
// queues as UnattachPending at the move itself for attachmentSBAs to publish
// (a flicker can return the bearer under the same ObjID before any SBA
// checkpoint; the fold leaves the live link standing, so only the queue can
// carry the departure across it) -- plus the bestowed detach (CR 702.114b)
// and rules' emit's re-attach detach (a re-attach to a different permanent).
// The second is the attachment's own departure from the battlefield (CR
// 701.3d, reached through the look-back walk whose split gate
// trigger_match.go's looksBackOnDeparture admits): a MoveZone off the
// battlefield of the attachment itself -- either still carrying its
// pre-departure AttachedTo (which the queue-marked link also is: the fold
// never cleared it), or carrying the fold's UnattachPending queue with
// LastBearer naming the bearer an earlier departure queued the detach from
// -- that bearer is the one it became unattached from. A departure whose LKI
// carries neither (an unattached Equipment destroyed) is no unattach at all
// and fires nothing.
//
// ValidAttachment$ names the ATTACHING object -- Forge's ValidSource$ for
// the detach half -- which is the object that actually became unattached:
// ev.Obj. It is evaluated against ev.Obj, NOT the trigger's source. Every
// corpus carrier spells it `Card.Self`, and Card.Self binds to the trigger's
// own source, so the two coincide exactly for those lines; but a face that
// pends several detaches on the battlefield must gate on the ONE attachment
// the event names, or every Grafted Exoskeleton present would fire when an
// unrelated Equipment detaches (the trigger would sacrifice that unrelated
// event's former bearer). ValidObject$ names the bearer the attachment
// became unattached from. On the bearer-left path that object has
// already left the battlefield, so matchesUnattachedBearer reads a bare
// `Permanent` base the way the rest of the engine reads a permanent CARD
// away from the battlefield (the leading-token `PermanentCard` rewrite),
// exactly as the Dig windows and the target census already do. A trigger
// naming neither parameter is treated as self-scoped: an attachment-only
// line fires for its own source rather than silently never firing.
func unattachedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	bearer, ok := unattachedBearer(ev, lki)
	if !ok {
		return false
	}
	if t.ParamStr(cards.PKStatic) == "True" {
		// Forge's "static effect expressed as a trigger" guard, the same one
		// attachedMatches carries: a clone/continuous ETB shape must not fire
		// on every detach.
		return false
	}
	ctrl := e.ControllerOf(source)
	if v, ok := t.Param(cards.PKValidAttachment); ok {
		// ev.Obj is the attachment the event names, exactly as
		// attachedMatches matches its ValidSource$ against ev.Obj. source is
		// what Card.Self binds to in specCtx, so the corpus's Card.Self lines
		// keep their meaning while an unrelated detach can no longer fire
		// this face.
		if !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
			return false
		}
	}
	if v, ok := t.Param(cards.PKValidObject); ok {
		return matchesUnattachedBearer(e, v, bearer, source, ctrl)
	}
	return true
}

// unattachedBearer selects the bearer a detach event names: an Unattached's
// IDs[0], or -- for the CR 701.3d departure arm -- the LKI's pre-departure
// AttachedTo on a MoveZone off the battlefield of the object itself. When the
// LKI instead carries the bearer-departure queue (the CR 400.7 fold queued
// this attachment's detach at an EARLIER move -- the bearer left first, the
// link stayed standing, and this MoveZone is the attachment's own departure
// before any SBA checkpoint published it), the queued LastBearer names the
// former bearer. ok is false for every other shape (including a departure
// with no bearer and no queue).
func unattachedBearer(ev events.Event, lki *state.Object) (state.ObjID, bool) {
	switch {
	case ev.Obj == 0:
		return 0, false
	case ev.Kind == events.Unattached:
		if len(ev.IDs) == 0 {
			return 0, false
		}
		return ev.IDs[0], true
	case ev.Kind == events.MoveZone && ev.From == state.ZBattlefield && ev.To != state.ZBattlefield:
		if lki == nil || lki.ID != ev.Obj {
			return 0, false
		}
		if lki.AttachedTo != 0 {
			return lki.AttachedTo, true
		}
		if lki.UnattachPending && lki.LastBearer != 0 {
			return lki.LastBearer, true
		}
		return 0, false
	}
	return 0, false
}

// matchesUnattachedBearer evaluates a Mode$ Unattached ValidObject$ spec
// against the former bearer. It first tries the ordinary live reading (the
// bearer is still a battlefield permanent on the protection / no-longer-a-
// creature / bestowed detach paths). When that fails and the bearer is no
// longer on the battlefield -- the bearer-left path, where the trigger must
// still fire for the permanent it WAS -- it retries with the leading
// `Permanent` base rewritten to `PermanentCard`, so a creature/battle/
// planeswalker card off the battlefield still matches Forge's "a permanent".
// The rewrite is inert for every non-Permanent base and for a live
// permanent (its face is a permanent type too), so the retry cannot widen a
// spec the live reading already decided.
func matchesUnattachedBearer(e Board, spec string, bearer, source state.ObjID, ctrl state.PlayerID) bool {
	if e.MatchesSpec(spec, bearer, source, ctrl, SpecOpts{}) {
		return true
	}
	o := e.Game().Obj(bearer)
	if o == nil || o.Zone == state.ZBattlefield {
		return false
	}
	return e.MatchesSpec(SpellCastPermanentSpec(spec), bearer, source, ctrl, SpecOpts{})
}

func init() {
	registerTrigMatcher(func(e Board, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return unattachedMatches(e, t, source, ev, lki)
	}, "Unattached")
}
