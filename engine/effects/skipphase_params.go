package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// skipPhaseParams is DB$ SkipPhase's parameter set, read once. Five shapes are
// modelled; every other has ok false and the effect says so with a Note:
//   - Phase$ BeginCombat, Start$ FromNext, Duration$
//     UntilTheEndOfTargetedNextTurn: all combat phases of the next turn (False
//     Peace, Empty City Ruse), scope 0;
//   - Step$ Untap or Draw with no Phase$, Start$ or Duration$: the seat's next
//     such step (Avizoa, Fatigue), scope state.SkipScopeNext;
//   - Step$ Draw, Duration$ EndOfTurn: this turn's draw step (Elfhame
//     Sanctuary), scope state.SkipScopeTurn;
//   - Phase$ BeginCombat with no Start$ or Duration$: the next combat phase
//     (Blinding Angel), scope state.SkipScopeNext;
//   - Phase$ BeginCombat, Duration$ NextThisTurn: the next combat phase this
//     turn (Moment of Silence), scope state.SkipScopeTurn.
type skipPhaseParams struct {
	step  state.Step // the skipped phase's first step
	scope state.ObjID
	ok    bool
	desc  string // the unmodelled shape, for the Note
}

func compileSkipPhaseParams(sa *cards.SA) skipPhaseParams {
	phase := strings.TrimSpace(sa.ParamStr(cards.PKPhase))
	start := strings.TrimSpace(sa.Params["Start"])
	dur := strings.TrimSpace(sa.ParamStr(cards.PKDuration))
	stepName := strings.TrimSpace(sa.Params["Step"])
	unmodelled := skipPhaseParams{desc: "Phase$ " + phase + " Step$ " + stepName + " Start$ " + start + " Duration$ " + dur}
	if stepName != "" {
		if phase != "" || start != "" {
			return unmodelled
		}
		step, ok := state.ParseStep(strings.ToLower(stepName))
		if !ok || (step != state.StepUntap && step != state.StepDraw) {
			return unmodelled
		}
		switch {
		case dur == "":
			return skipPhaseParams{step: step, scope: state.SkipScopeNext, ok: true}
		case dur == "EndOfTurn" && step == state.StepDraw:
			return skipPhaseParams{step: step, scope: state.SkipScopeTurn, ok: true}
		}
		return unmodelled
	}
	if phase != "BeginCombat" {
		return unmodelled
	}
	switch {
	case start == "FromNext" && dur == "UntilTheEndOfTargetedNextTurn":
		return skipPhaseParams{step: state.StepBeginCombat, ok: true}
	case start == "" && dur == "":
		return skipPhaseParams{step: state.StepBeginCombat, scope: state.SkipScopeNext, ok: true}
	case start == "" && dur == "NextThisTurn":
		return skipPhaseParams{step: state.StepBeginCombat, scope: state.SkipScopeTurn, ok: true}
	}
	return unmodelled
}
