package rules

import (
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// emergeCaptureEvent is the Emerge (CR 702.118a) capture: the sacrificed
// creature's toughness stored on the spell as a named runtime SVar
// (events.StoreSVar, the capture Count$NotedNumber's "note a number onto the
// card, read it at the later ETB" shape). Count$Emerged's <yes> operand and
// the Emerged$ CardToughness body read it (Adipose Offspring's "create X of
// those tokens, where X is the sacrificed creature's toughness"). It is
// returned ONLY when the sacrifice part of the emerge cost was actually paid
// and the face carries a reader of the head (faceWantsEmerged), so every
// other emerge cast -- and every ordinary cast -- stays byte-identical: an
// over-charged emerge (sacrifice declined) reads "not emerged", and so does
// a copy (a separate object was never cast, CR 707.10). The value survives
// the stack->battlefield move and events.Move's leave-the-battlefield reset
// clears it with the cast-time window (CR 400.7), so a re-entering permanent
// reads "not emerged".
func emergeCaptureEvent(g *state.Game, pc *pendingCast, sacrificed []state.SacrificedInfo) (events.Event, bool) {
	if !pc.emerge || pc.emergeSac == 0 || !faceWantsEmerged(g.Obj(pc.card).Face()) {
		return events.Event{}, false
	}
	for _, si := range sacrificed {
		if si.Obj == pc.emergeSac {
			return events.Event{Kind: events.StoreSVar, Obj: pc.card,
				Text: effects.EmergedSacToughnessSVar, Amount: si.Toughness}, true
		}
	}
	return events.Event{}, false
}
