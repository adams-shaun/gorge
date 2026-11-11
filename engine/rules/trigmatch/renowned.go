package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// becomeRenownedMatches implements Mode$ BecomeRenowned ("When CARDNAME
// becomes renowned", Relic Seeker; "Whenever a creature you control becomes
// renowned", Valeron Wardens). CR 702.112b: a permanent becomes renowned as
// its renown ability resolves, which effPutCounter marks with an
// events.AlterAttribute Text "Renowned" (emitted only when counters were
// actually put, and at most once while the creature stays renowned). The
// mark's Obj is the creature that became renowned; ValidCard$ is tested
// against it, the becomeMonstrousMatches shape.
func becomeRenownedMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if !events.IsAlterAttribute(ev, "Renowned") || ev.Amount < 1 {
		return false
	}
	ctrl := e.ControllerOf(source)
	if v := t.ParamStr(cards.PKValidCard); v != "" && !e.MatchesSpec(v, ev.Obj, source, ctrl, SpecOpts{}) {
		return false
	}
	return true
}

func init() {
	registerTrigMatcher(becomeRenownedMatches, "BecomeRenowned")
	effects.RegisterNonAPI("trig:BecomeRenowned")
}
