package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

func init() { Register("GameDrawn", effGameDrawn) }

// effGameDrawn implements DB$ GameDrawn (Forge's GameDrawEffect; Celestial
// Convergence and Divine Intervention's "the game is a draw"): the game ends
// with no winner, CR 104.4a. The one GameOver draw shape events.Apply's fold
// defines (Amount 1, Player irrelevant) is proposed directly through Emit:
// unlike the win and loss sides, a draw has no CantHappen replacement family
// in the corpus (every R:Event$ GameWin/GameLoss gate routes through
// rules' EmitGameWin/EmitPlayerLost funnels, which a draw has no member of),
// so there is no rules-side gate to reach and no Host method to add. A game
// that has already ended is skipped, exactly the effWinsGame guard.
//
// The gating ConditionPresent$ / ConditionCheckSVar$ / ConditionSVarCompare$
// keys are consumed by the shared conditionMet gate ahead of the body.
func effGameDrawn(h Host, c *Ctx, sa *cards.SA) {
	if h.Game().Over {
		return
	}
	h.Emit(events.Event{Kind: events.GameOver, Amount: 1})
}
