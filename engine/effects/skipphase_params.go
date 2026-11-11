package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// skipPhaseParams is DB$ SkipPhase's parameter set, read once. Only the
// "skips all combat phases of their next turn" shape is modelled (Phase$
// BeginCombat, Start$ FromNext, Duration$ UntilTheEndOfTargetedNextTurn:
// False Peace, Empty City Ruse); every other shape has ok false and the
// effect says so with a Note.
type skipPhaseParams struct {
	step state.Step // the skipped phase's first step
	ok   bool
	desc string // the unmodelled shape, for the Note
}

func compileSkipPhaseParams(sa *cards.SA) skipPhaseParams {
	phase := strings.TrimSpace(sa.ParamStr(cards.PKPhase))
	start := strings.TrimSpace(sa.Params["Start"])
	dur := strings.TrimSpace(sa.ParamStr(cards.PKDuration))
	if step := strings.TrimSpace(sa.Params["Step"]); step != "" {
		return skipPhaseParams{desc: "Step$ " + step}
	}
	if phase != "BeginCombat" || start != "FromNext" || dur != "UntilTheEndOfTargetedNextTurn" {
		return skipPhaseParams{desc: "Phase$ " + phase + " Start$ " + start + " Duration$ " + dur}
	}
	return skipPhaseParams{step: state.StepBeginCombat, ok: true}
}
