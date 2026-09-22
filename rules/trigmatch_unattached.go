package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// unattachedMatches implements Mode$ Unattached (CR 701.3b), the
// "whenever CARDNAME becomes unattached from a permanent" trigger the four
// corpus carriers print (Captain's Hook, Grafted Exoskeleton, Grafted
// Wargear, Stitcher's Graft -- all Equipment). The four lines share one
// shape: `ValidAttachment$ Card.Self | ValidObject$ Permanent`.
//
// The event is events.Unattached (NOT an empty-IDs events.Attach): a distinct
// Kind is what lets this mode fire while Mode$ Attached keeps ignoring a
// detach, and the event carries the attachment in Obj and the FORMER BEARER
// in IDs[0]. rules/attach.go's attachmentSBAs is the one emit site for the
// arms where the attachment stays on the battlefield (the bearer left, the
// bearer stopped being a valid bearer, protection, the CR 702.114b bestowed
// type switch) -- the same detach sites its own comment enumerates.
//
// ValidAttachment$ names the ATTACHING object, which is the trigger's own
// source (Card.Self on every carrier), so it is evaluated against source.
// ValidObject$ names the bearer the attachment became unattached from:
// ev.IDs[0]. On the bearer-left path that object has already left the
// battlefield, so matchesUnattachedBearer reads a bare `Permanent` base the
// way the rest of the engine reads a permanent CARD away from the
// battlefield (the leading-token `PermanentCard` rewrite), exactly as the
// Dig windows and the target census already do. A trigger naming neither
// parameter is treated as self-scoped: an attachment-only line fires for its
// own source rather than silently never firing.
func (e *Engine) unattachedMatches(t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
	if ev.Kind != events.Unattached || ev.Obj == 0 || len(ev.IDs) == 0 {
		return false
	}
	if t.Params["Static"] == "True" {
		// Forge's "static effect expressed as a trigger" guard, the same one
		// attachedMatches carries: a clone/continuous ETB shape must not fire
		// on every detach.
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidAttachment"]; ok {
		if !effects.MatchesSpecCtx(e.G, v, source, e.specCtx(source, ctrl)) {
			return false
		}
	}
	if v, ok := t.Params["ValidObject"]; ok {
		return e.matchesUnattachedBearer(v, ev.IDs[0], source, ctrl)
	}
	return true
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
func (e *Engine) matchesUnattachedBearer(spec string, bearer, source state.ObjID, ctrl state.PlayerID) bool {
	sc := e.specCtx(source, ctrl)
	if effects.MatchesSpecCtx(e.G, spec, bearer, sc) {
		return true
	}
	o := e.G.Obj(bearer)
	if o == nil || o.Zone == state.ZBattlefield {
		return false
	}
	return effects.MatchesObjectCtx(e.G, spellCastPermanentSpec(spec), o, sc)
}

func init() {
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, lki *state.Object) bool {
		return e.unattachedMatches(t, source, ev, lki)
	}, "Unattached")
}
