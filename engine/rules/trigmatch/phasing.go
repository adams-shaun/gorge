package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// PhaseInMatches implements Mode$ PhaseIn ("Whenever ~ phases in ...", CR
// 702.26; Shimmering Efreet, Warping Wurm, King of the Oathbreakers). The
// events.PhaseOut marker with Amount -1 is the phase-in.
func PhaseInMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	return phasingMatches(e, t, source, ev, false)
}

// PhaseOutMatches implements Mode$ PhaseOut ("Whenever ~ phases out ...",
// Teferi's Imp, Ertai's Familiar): the marker with Amount >= 1. The source is
// phased out when this runs, so ValidCard$ is read against the object as it
// now is (`Card.phasedOutSelf`, the CR 702.26b look-back); the engine admits a
// phased-out source to the trigger walk only for this mode.
func PhaseOutMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	return phasingMatches(e, t, source, ev, true)
}

// phasingMatches reports whether the permanent the event names, or one that
// phased with it (CR 702.26g/h), passes t's ValidCard$ filter (absent: any).
// The fold derives the indirect phasing from the ONE PhaseOut event of the
// host, so the attached set is walked here off the folded state instead of a
// second event kind: on a phase-out the attachments that are now out
// indirectly, on a phase-in the attachments that are now in (every
// attachment of a phased-out host was out until this event). A batch is one
// trigger per source and event, as for every other single-event matcher.
func phasingMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, out bool) bool {
	if ev.Kind != events.PhaseOut || ev.Obj == 0 || (ev.Amount >= 1) != out {
		return false
	}
	spec := t.ParamStr(cards.PKValidCard)
	ctrl := e.ControllerOf(source)
	ok := func(id state.ObjID) bool {
		return spec == "" || e.MatchesSpec(spec, id, source, ctrl, SpecOpts{})
	}
	if ok(ev.Obj) {
		return true
	}
	return anyPhasedWith(e.Game(), ev.Obj, out, ok)
}

// anyPhasedWith walks the battlefield permanents attached to host (and their
// attachments) that phased with it, in arena order, until ok accepts one.
func anyPhasedWith(g *state.Game, host state.ObjID, out bool, ok func(state.ObjID) bool) bool {
	for i := range g.Objs {
		a := &g.Objs[i]
		if a.Zone != state.ZBattlefield || a.AttachedTo != host || a.PhasedOut != out || out && !a.PhasedOutIndirect {
			continue
		}
		if ok(a.ID) || anyPhasedWith(g, a.ID, out, ok) {
			return true
		}
	}
	return false
}

func init() {
	registerTrigMatcher(PhaseInMatches, "PhaseIn")
	registerTrigMatcher(PhaseOutMatches, "PhaseOut")
}
