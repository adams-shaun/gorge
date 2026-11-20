package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// The scope of an ExtSkipPhase C (bits 4-5): gorge state.SkipScopeNext and
// SkipScopeTurn.
const (
	skipScopeNext = 1
	skipScopeTurn = 2
)

// skipC is the ExtSkipPhase C of a one-shot grant of step: the step in bits 0-3
// and the scope above it.
func skipC(step kop.Step, scope int) int { return scope<<4 | int(step) }

// DB$ SkipPhase (gorge effSkipPhase, CR 614.10) lowers as OpExt ExtSkipPhase
// over the skipping player (Obj), C the grant (skipC; zero all combat phases of
// the next turn), in the shapes
// gorge models (engine/effects/skipphase_params.go):
//   - Phase$ BeginCombat | Start$ FromNext | Duration$
//     UntilTheEndOfTargetedNextTurn: all combat phases of the next turn (False
//     Peace, Empty City Ruse);
//   - Step$ Untap | Draw with no Phase$/Start$/Duration$: the next such step
//     (Avizoa, Fatigue); Step$ Draw | Duration$ EndOfTurn: this turn's draw
//     step (Elfhame Sanctuary);
//   - Phase$ BeginCombat with no Start$/Duration$: the next combat phase (Blinding
//     Angel); Duration$ NextThisTurn: the next combat phase this turn (Moment of
//     Silence).
//
// Any other shape refuses by name.
func (lc *lowerCtx) skipPhase(ps *params) error {
	grant, ok := skipPhaseGrant(ps)
	if !ok {
		return refuse("SkipPhase Phase$ %q Start$ %q Duration$ %q Step$ %q (only the modelled shapes lower: all combat phases of the next turn, the next untap/draw step, the next combat phase)",
			ps.str("Phase"), ps.str("Start"), ps.str("Duration"), ps.str("Step"))
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rTriggeredPlayer: // the opponent (two seats), the trigger's player
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("SkipPhase of a non-player target")
		}
	default:
		return refuse("SkipPhase of %s", who)
	}
	lc.emit(lInstr{Op: "SkipPhase", Obj: who, C: grant, Note: "the player skips the grant's phase or step (gorge effSkipPhase)"})
	return nil
}

// skipPhaseGrant reads the shape: the ExtSkipPhase C, false for a shape
// gorge's compileSkipPhaseParams does not model either.
func skipPhaseGrant(ps *params) (int, bool) {
	phase, start, dur, step := ps.str("Phase"), ps.str("Start"), ps.str("Duration"), ps.str("Step")
	switch {
	case step == "Untap" && phase == "" && start == "" && dur == "":
		return skipC(kop.StepUntap, skipScopeNext), true
	case step == "Draw" && phase == "" && start == "" && dur == "":
		return skipC(kop.StepDraw, skipScopeNext), true
	case step == "Draw" && phase == "" && start == "" && dur == "EndOfTurn":
		return skipC(kop.StepDraw, skipScopeTurn), true
	case step != "" || phase != "BeginCombat":
		return 0, false
	case start == "FromNext" && dur == "UntilTheEndOfTargetedNextTurn":
		return 0, true
	case start == "" && dur == "":
		return skipC(kop.StepBeginCombat, skipScopeNext), true
	case start == "" && dur == "NextThisTurn":
		return skipC(kop.StepBeginCombat, skipScopeTurn), true
	}
	return 0, false
}
