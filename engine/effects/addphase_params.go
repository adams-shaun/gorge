package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// addPhaseGatedByPostCombatMain reports whether an AddPhase grant is refused
// by BeforeFirstPostCombatMainEnd$ True (World at War, the corpus's only
// carrier): Forge's AddPhaseEffect returns before granting when the first
// postcombat main phase has already ended. Forge's test
// (PhaseHandler.beforeFirstPostCombatMainEnd) is nMainsThisTurn <= (is(MAIN2)
// ? 2 : 1): the precombat main (1) and the first postcombat main (2) pass, an
// extra main phase another AddPhase created (3 and up) does not.
func addPhaseGatedByPostCombatMain(g *state.Game, sa *cards.SA) bool {
	if !isTrue(sa.Params["BeforeFirstPostCombatMainEnd"]) {
		return false
	}
	limit := int32(1)
	if g.Step == state.StepMain2 {
		limit = 2
	}
	return g.MainPhasesThisTurn > limit
}
