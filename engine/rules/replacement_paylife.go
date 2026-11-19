package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() { effects.RegisterNonAPI("repl:PayLife") }

// payLifeProposalDispatcher carries the two engine entry points the marked
// settlement's emit boundary needs: the replacement dispatch for the
// positive proposal and the trigger walk for it (the existing checkTriggers,
// reached through this interface so neither function grows the engine's
// measured surface).
type payLifeProposalDispatcher interface {
	applyReplacementsDispatch(events.Event) (events.Event, bool)
	checkTriggers(ev *events.Event, lki *state.Object, lkiPower, lkiToughness int32, lkiPTValid bool)
}

// interceptPayLifeProposal turns a marked cost settlement into an unlogged,
// positive-amount proposal before the negative LifeChange reaches the fold.
func prepareEmitReplacement(dispatch payLifeProposalDispatcher, ev *events.Event, applying bool,
	action string, replaced state.ObjID) bool {
	if interceptPayLifeProposal(dispatch, ev) {
		return true
	}
	if applying {
		*ev = events.CarryAction(action, replaced, *ev)
	}
	return false
}

func interceptPayLifeProposal(dispatch payLifeProposalDispatcher, ev *events.Event) bool {
	if ev.Kind != events.LifeChange || ev.Text != pay.PayLifeProposalText || ev.Amount >= 0 {
		return false
	}
	proposal := *ev
	proposal.Amount = -proposal.Amount
	handled := false
	if _, h := dispatch.applyReplacementsDispatch(proposal); h {
		handled = true
	}
	// trig:PayLife (Font of Agonies, edh-trigB) triggers off the same
	// proposal when the payment settles. The cost was paid (CR 118.5 -- a
	// cost payment is not an event), but a replacement that CONSUMED the
	// proposal -- a CantLoseLife/CantChangeLife prevention, a wholly
	// replacing body, a parked CR 616.1 order choice -- un-happens the loss,
	// and the mode reads the settled loss exactly like any LifeChange
	// trigger does. The proposal is unlogged and never folded, so the
	// ordinary emit-tail trigger check can never see it; this queue-side
	// check is the mode's only firing site. A replacement's own resolution
	// above has already emitted and queued what it emits, so the payment
	// trigger follows it in the queue.
	if !handled {
		dispatch.checkTriggers(&proposal, nil, 0, 0, false)
	}
	// The plain negative loss folds without the proposal mark (the contract
	// at events.PayLifeProposalText), so the mark is never part of the
	// replay log: the walk above reads the copied proposal, so the mark is
	// cleared on the original before the caller carries and folds it.
	ev.Text = ""
	return handled
}

type lifeReplacementMatcher interface {
	Game() *state.Game
	replCtx(replMatch, events.Event) *effects.Ctx
	replacementAmountMatches(string, int32, *effects.Ctx) bool
	replacementConditionHolds(cards.Repl, state.ObjID, state.PlayerID) bool
}

func gainLifeReplacementMatches(e lifeReplacementMatcher, r cards.Repl, source state.ObjID, ev events.Event,
	rememberedPlayers []state.PlayerID, you state.PlayerID) bool {
	if ev.Kind != events.LifeChange || ev.Amount <= 0 {
		return false
	}
	if vp := strings.TrimSpace(r.ParamStr(cards.PKValidPlayer)); vp != "" {
		if vp == "Player.IsRemembered" {
			found := false
			for _, p := range rememberedPlayers {
				if p == ev.Player {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		} else if !effects.MatchesPlayerSpec(e.Game(), vp, ev.Player, you) {
			return false
		}
	}
	return e.replacementConditionHolds(r, source, you)
}

func payLifeReplacementMatches(e lifeReplacementMatcher, r cards.Repl, source state.ObjID, ev events.Event, you state.PlayerID) bool {
	if ev.Kind != events.LifeChange || ev.Text != pay.PayLifeProposalText || ev.Amount <= 0 {
		return false
	}
	if vp, ok := r.Param(cards.PKValidPlayer); ok &&
		!effects.MatchesPlayerSpec(e.Game(), vp, ev.Player, you) {
		return false
	}
	ctx := e.replCtx(replMatch{id: source, repl: &r}, ev)
	return e.replacementAmountMatches(r.ParamStr(cards.PKAmount), ev.Amount, ctx) &&
		e.replacementConditionHolds(r, source, you)
}
