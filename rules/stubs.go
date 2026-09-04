package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// checkStateBased runs the CR 704 state-based actions this build implements:
// destroyLethalDamage (rules/combat.go, Task 21 -- lethal/zero-toughness
// creature destruction; nothing checked this before, so a creature damaged by
// an ordinary spell, not only in combat, never actually died either), then
// the game-over check that was already here.
func (e *Engine) checkStateBased() {
	e.destroyLethalDamage()
	e.checkGameOver()
}

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
