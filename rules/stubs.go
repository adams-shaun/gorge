package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Replaced in Tasks 21 and 22.
func (e *Engine) handleAttackers(*decision.Decision, decision.Intent) {}
func (e *Engine) handleBlockers(*decision.Decision, decision.Intent)  {}
func (e *Engine) askAttackers()                                       { e.setStep(state.StepEndCombat) }
func (e *Engine) askBlockers()                                        { e.setStep(state.StepCombatDamage) }
func (e *Engine) dealCombatDamage()                                   {}

func (e *Engine) checkStateBased() { e.checkGameOver() }

func (e *Engine) checkGameOver() {
	alive := e.G.AliveFrom(0)
	if len(alive) > 1 || e.G.Over {
		return
	}
	w := state.PlayerID(0)
	if len(alive) == 1 {
		w = alive[0]
	}
	e.emit(events.Event{Kind: events.GameOver, Player: w})
	e.pending = nil
}
