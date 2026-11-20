package rules

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// skipPendingStep is the consumer of DB$ SkipPhase's one-shot grants (CR
// 614.10: a skipped step or phase does not happen). A walk that would enter
// the draw step while the active player has SkipDrawPending enters the
// precombat main phase instead; one that would enter the beginning of combat
// while the active player has SkipNextCombat or SkipCombatThisTurn enters the
// postcombat main phase. Each skip is consumed by one SkipPhase event with
// Amount -1. A live all-combats skip (skipCombatPhase) takes the combat phase
// first and leaves the one-shot bits for the next. Free function over the
// game and the emit, like expireCombatControl.
func skipPendingStep(g *state.Game, emit func(events.Event) events.Event, next state.Step) state.Step {
	bits := g.PhaseSkips[g.Active]
	switch {
	case next == state.StepDraw && bits&state.SkipDrawPending != 0:
		emit(events.Event{Kind: events.SkipPhase, Player: g.Active, Step: state.StepDraw, Amount: -1})
		return state.StepMain1
	case next == state.StepBeginCombat && bits&(state.SkipNextCombat|state.SkipCombatThisTurn) != 0:
		emit(events.Event{Kind: events.SkipPhase, Player: g.Active, Step: state.StepBeginCombat, Amount: -1})
		return state.StepMain2
	}
	return next
}

// skipUntapStep reports whether the turn that just began for seat skips its
// untap step (SkipUntapPending), consuming the grant. CR 614.10a: the skipped
// step untaps nothing and phases nothing; the turn enters its upkeep step.
func skipUntapStep(g *state.Game, emit func(events.Event) events.Event, seat state.PlayerID) bool {
	if g.PhaseSkips[seat]&state.SkipUntapPending == 0 {
		return false
	}
	emit(events.Event{Kind: events.SkipPhase, Player: seat, Step: state.StepUntap, Amount: -1})
	return true
}
