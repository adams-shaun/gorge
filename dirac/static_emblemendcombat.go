package dirac

// DebugNoEmbEndCombat (the lockstep mutation): an end-of-combat emblem
// (kop.Emblem.UntilEndCombat) is not dropped on the way out of the
// end-of-combat step, so it lasts to the cleanup that sweeps it.
var DebugNoEmbEndCombat bool

// dropEOCEmblems drops every emblem of a Duration$ UntilEndOfCombat Effect on
// the way out of the end-of-combat step (gorge turn.go finishStepBoundary ->
// expireUntilEndOfCombat, CR 511.3). A later extra combat does not revive it:
// the emblem is gone. An emblem made after the end-of-combat step, or in a
// turn with no combat, stays to the cleanup (kop.Emblem.UntilEOT is set beside
// UntilEndCombat; dropEOTEmblems).
func (s *State) dropEOCEmblems(k *prep) {
	if DebugNoEmbEndCombat {
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		var drop uint8
		for i := 0; i < int(s.g.nemb[p]); i++ {
			if k.t.Emblems[s.embIdx(p, i)-1].UntilEndCombat {
				drop |= 1 << uint(i)
			}
		}
		if drop != 0 {
			s.embCompact(p, drop)
		}
	}
}
