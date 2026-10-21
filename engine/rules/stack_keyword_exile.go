package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// keywordExileDeparture applies the stack half of flashback (CR 702.34a),
// aftermath (CR 702.127a), jump-start (CR 702.133a) and harmonize
// (CR 702.180a): a spell cast that way is exiled "instead of putting it
// anywhere else any time it would leave the stack". Every departure from the
// stack funnels through emit, so the rewrite lives here once: a counter's
// named destination (Remand's hand, Memory Lapse's library), a ChangeZone
// bounce of a spell (Unsubstantiate) and the resolution/fizzle rest zones all
// end in exile. The one exception is a CR 733.1 reversal of the cast still
// being proposed: the spell was never cast, so it returns to where it came
// from (abortCast emits that move while e.cast is still the proposal).
func (e *Engine) keywordExileDeparture(ev *events.Event) {
	if ev.Kind != events.MoveZone || ev.From != state.ZStack || ev.To == state.ZExile {
		return
	}
	o := e.G.Obj(ev.Obj)
	if o == nil || o.Zone != state.ZStack || o.Ability != nil || !state.ExilesLeavingStack(o.CastFlags) {
		return
	}
	if e.cast != nil && e.cast.stackObj == ev.Obj {
		return
	}
	ev.To = state.ZExile
}
