package rules

import (
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// legalActions is replaced in Task 13. Until then every player may only pass,
// which is exactly right for the priority and turn-order tests.
func (e *Engine) legalActions(p state.PlayerID) []decision.Option {
	return []decision.Option{{Index: 0, Kind: "pass", Label: "Pass priority"}}
}

func (e *Engine) handlePriority(d *decision.Decision, in decision.Intent) {
	if d.Chosen(in)[0].Kind != "pass" {
		return
	}
	e.G.Passes++
	if e.G.Passes >= int32(e.G.AliveCount()) {
		e.G.Passes = 0
		if len(e.G.Stack) > 0 {
			e.resolveTop()
			e.G.Priority = e.G.Active
			return
		}
		e.advanceStep()
		return
	}
	e.G.Priority = e.G.NextAlive(e.G.Priority)
}

// Replaced in Tasks 14, 21 and 22.
func (e *Engine) handleTarget(*decision.Decision, decision.Intent)    {}
func (e *Engine) handleAttackers(*decision.Decision, decision.Intent) {}
func (e *Engine) handleBlockers(*decision.Decision, decision.Intent)  {}
func (e *Engine) resolveTop()                                         {}
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
