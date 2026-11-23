package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// skipPhaseParams is DB$ SkipPhase's parameter set, read once. The shapes
// modelled (Forge's SkipPhaseEffect, CR 614.10):
//
//   - Phase$ BeginCombat, Start$ FromNext, Duration$
//     UntilTheEndOfTargetedNextTurn: every combat phase of the next turn
//     (False Peace, Empty City Ruse), amount 0;
//   - Phase$ BeginCombat with no Start$ or Duration$: the next combat phase,
//     this turn when it is still to come (Blinding Angel, Revenant Patriarch,
//     Stonehorn Dignitary), amount 1;
//   - Phase$ BeginCombat, Duration$ NextThisTurn: the next combat phase this
//     turn only (Moment of Silence), amount 2;
//   - Step$ Untap with no Duration$: the next untap step (Avizoa, Brine
//     Elemental, Dovin, Shisato), amount 0;
//   - Step$ Draw with no Duration$ or Duration$ EndOfTurn: the next draw step
//     (Fatigue, Ivory Gargoyle, Molten Firebird; Elfhame Sanctuary's grant
//     comes in the upkeep, so its "this turn" is the next draw step too).
//
// Every other shape has ok false and the effect says so with a Note.
type skipPhaseParams struct {
	step   state.Step // the skipped phase's first step
	amount int32      // events.SkipPhase Amount: the combat shape, 0 otherwise
	ok     bool
	desc   string // the unmodelled shape, for the Note
}

// skipPhaseShapes maps the Step$|Phase$|Start$|Duration$ key of a modelled
// shape to its parameters (a table, not a word dispatch).
var skipPhaseShapes = map[string]skipPhaseParams{
	"Untap|||":         {step: state.StepUntap, ok: true},
	"Draw|||":          {step: state.StepDraw, ok: true},
	"Draw|||EndOfTurn": {step: state.StepDraw, ok: true},
	"|BeginCombat|FromNext|UntilTheEndOfTargetedNextTurn": {step: state.StepBeginCombat, ok: true},
	"|BeginCombat||":             {step: state.StepBeginCombat, amount: 1, ok: true},
	"|BeginCombat||NextThisTurn": {step: state.StepBeginCombat, amount: 2, ok: true},
}

func compileSkipPhaseParams(sa *cards.SA) skipPhaseParams {
	phase := strings.TrimSpace(sa.ParamStr(cards.PKPhase))
	start := strings.TrimSpace(sa.Params["Start"])
	dur := strings.TrimSpace(sa.ParamStr(cards.PKDuration))
	step := strings.TrimSpace(sa.Params["Step"])
	if p, ok := skipPhaseShapes[step+"|"+phase+"|"+start+"|"+dur]; ok {
		return p
	}
	return skipPhaseParams{desc: "Step$ " + step + " Phase$ " + phase + " Start$ " + start + " Duration$ " + dur}
}
