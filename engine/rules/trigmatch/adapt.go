package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// adaptMatches implements Mode$ Adapt ("Whenever a creature adapts", CR
// 701.46): the triggering event is the AlterAttribute marker effects'
// putCounterAdaptMark emits once an Adapt$ put has landed its counters, whose
// Obj is the creature that adapted. ValidCard$ is tested against it, the
// becomeRenownedMatches shape. The corpus's only carrier is Biomancer's
// Familiar's Effect trigger (`ValidCard$ Card.IsRemembered`, Static$ True, its
// body self-exiling the Effect), registered through effEffect's event-mode
// arm.
func adaptMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if !events.IsAlterAttribute(ev, effects.AdaptedMark) {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

func init() {
	registerTrigMatcher(adaptMatches, "Adapt")
	effects.RegisterNonAPI("trig:Adapt")
}
