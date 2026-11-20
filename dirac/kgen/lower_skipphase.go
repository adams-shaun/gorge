package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// DB$ SkipPhase (gorge effSkipPhase, CR 614.10) lowers as OpExt ExtSkipPhase
// over the skipping player(s) (Obj), C the shape gorge's compileSkipPhaseParams
// models:
//
//   - Phase$ BeginCombat | Start$ FromNext | Duration$ UntilTheEndOfTargetedNextTurn:
//     all combat phases of the next turn (False Peace, Empty City Ruse);
//   - Phase$ BeginCombat with no Start$ or Duration$: the next combat phase
//     (Blinding Angel, Revenant Patriarch, Stonehorn Dignitary);
//   - Phase$ BeginCombat | Duration$ NextThisTurn: the next combat phase this
//     turn only (Moment of Silence);
//   - Step$ Untap with no Duration$: the next untap step (Avizoa, Brine
//     Elemental, Dovin, Shisato);
//   - Step$ Draw with no Duration$ or Duration$ EndOfTurn: the next draw step
//     (Fatigue, Ivory Gargoyle, Molten Firebird, Elfhame Sanctuary).
//
// Every other shape is a Note in gorge and refuses by name here.
func (lc *lowerCtx) skipPhase(ps *params) error {
	shape, ok := skipPhaseShape(ps)
	if !ok {
		return refuse("SkipPhase Phase$ %q Start$ %q Duration$ %q Step$ %q (not a shape gorge's SkipPhase models)",
			ps.str("Phase"), ps.str("Start"), ps.str("Duration"), ps.str("Step"))
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou, rEachOpp, rTriggeredPlayer:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("SkipPhase of a non-player target")
		}
	default:
		return refuse("SkipPhase of %s", who)
	}
	lc.emit(lInstr{Op: "SkipPhase", Obj: who, C: shape, Note: "the player skips the phase or step the shape names (gorge effSkipPhase)"})
	return nil
}

// skipPhaseShapes is gorge's skipPhaseShapes table (Step$|Phase$|Start$|Duration$)
// as ExtSkipPhase's C.
var skipPhaseShapes = map[string]int{
	"Untap|||":         kop.SkipPhaseUntap,
	"Draw|||":          kop.SkipPhaseDraw,
	"Draw|||EndOfTurn": kop.SkipPhaseDraw,
	"|BeginCombat|FromNext|UntilTheEndOfTargetedNextTurn": kop.SkipPhaseAllCombats,
	"|BeginCombat||":             kop.SkipPhaseNextCombat,
	"|BeginCombat||NextThisTurn": kop.SkipPhaseThisTurn,
}

func skipPhaseShape(ps *params) (int, bool) {
	shape, ok := skipPhaseShapes[ps.str("Step")+"|"+ps.str("Phase")+"|"+ps.str("Start")+"|"+ps.str("Duration")]
	return shape, ok
}
