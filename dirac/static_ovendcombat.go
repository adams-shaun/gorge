package dirac

// DebugNoOvEndCombat (the lockstep mutation): an end-of-combat override
// record (kop.Override.UntilEndCombat) is not dropped on the way out of the
// end-of-combat step.
var DebugNoOvEndCombat bool

// ovCleanupEnds reports whether the cleanup step ends override record r: an
// until-end-of-turn record, or an until-end-of-combat one that no end-of-combat
// step took (a skipped combat, a record made in the postcombat main phase;
// gorge's cleanup sweep drops that class too).
func ovCleanupEnds(k *prep, r *fxRec) bool {
	return r.kind == fxOverride && (r.r.eot() || ovUntilEndCombat(k, r))
}

// ovUntilEndCombat reports whether override record r is of the
// Duration$ UntilEndOfCombat class. The class lives on the static row, not
// the record: ovRec.fl has no free bit, and the record is applied with
// DurPermanent so the end-of-turn drop leaves it alone.
func ovUntilEndCombat(k *prep, r *fxRec) bool {
	return r.kind == fxOverride && r.r.idx > 0 && k.t.Overrides[r.r.idx-1].UntilEndCombat
}

// ovEndCombat drops every Duration$ UntilEndOfCombat override record on the
// way out of the end-of-combat step (gorge turn.go finishStepBoundary ->
// expireUntilEndOfCombat, after the "at end of combat" triggers; CR 511.3).
// A later extra combat does not revive it: the record is gone, not paused.
func (s *State) ovEndCombat(k *prep) {
	if DebugNoOvEndCombat || s.g.nfx == 0 {
		return
	}
	any := false
	for i := 0; i < int(s.g.nfx); i++ {
		if ovUntilEndCombat(k, &s.g.fx[i]) {
			any = true
			break
		}
	}
	if !any {
		return
	}
	s.dropFx(func(i int) bool { return ovUntilEndCombat(k, &s.g.fx[i]) })
	s.refreshTy(k) // as hostLeft does after its dropFx
}
