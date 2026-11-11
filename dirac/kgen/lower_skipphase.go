package kgen

// DB$ SkipPhase (gorge effSkipPhase, CR 614.10) lowers as OpExt ExtSkipPhase
// over the skipping player (Obj), in the one shape gorge models: Phase$
// BeginCombat | Start$ FromNext | Duration$ UntilTheEndOfTargetedNextTurn, "skips
// all combat phases of their next turn" (False Peace, Empty City Ruse). The
// one-shot "next combat phase this turn" (Moment of Silence), the Defined$ form
// without a duration (Blinding Angel) and Step$ skips are not modelled in
// gorge either and refuse by name.
func (lc *lowerCtx) skipPhase(ps *params) error {
	if !ps.is("Phase", "BeginCombat") || !ps.is("Start", "FromNext") || !ps.is("Duration", "UntilTheEndOfTargetedNextTurn") {
		return refuse("SkipPhase Phase$ %q Start$ %q Duration$ %q Step$ %q (only all combat phases of the next turn lowers)",
			ps.str("Phase"), ps.str("Start"), ps.str("Duration"), ps.str("Step"))
	}
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	switch who.Kind {
	case rYou:
	case rTarget:
		if t := lc.ab.Targets[who.N]; t.F == nil || t.F.Players == 0 || len(t.F.Clauses) != 0 {
			return refuse("SkipPhase of a non-player target")
		}
	default:
		return refuse("SkipPhase of %s", who)
	}
	lc.emit(lInstr{Op: "SkipPhase", Obj: who, Note: "the player skips all combat phases of their next turn (gorge effSkipPhase)"})
	return nil
}
