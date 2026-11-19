package trigmatch

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// payLifeMatches implements Mode$ PayLife ("Whenever you pay life", Font of
// Agonies). The triggering event is the unlogged POSITIVE pay-life proposal
// rules' interceptPayLifeProposal dispatches from the marked negative
// LifeChange every cost settlement emits: Amount is the life PAID (the
// negative loss's magnitude), Text is the events.PayLifeProposalText marker,
// Player the payer. The plain negative loss that folds afterwards carries no
// marker, so a PayLife line never fires on the loss a replacement or
// game rule caused -- paying life is a cost settlement (CR 118.5), which is
// exactly what the marker means. ValidPlayer$ names the payer the same way
// lifeLostMatches reads it.
func payLifeMatches(e Board, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
	if ev.Kind != events.LifeChange || ev.Amount <= 0 || ev.Text != events.PayLifeProposalText {
		return false
	}
	p := ev.Player
	if int(p) < 0 || int(p) >= len(e.Game().Players) {
		return false
	}
	if v, ok := t.Param(cards.PKValidPlayer); ok &&
		!effects.MatchesPlayerSpec(e.Game(), v, p, e.ControllerOf(source)) {
		return false
	}
	return true
}

func init() {
	registerTrigMatcher(payLifeMatches, "PayLife")
	effects.RegisterNonAPI("trig:PayLife")
}
