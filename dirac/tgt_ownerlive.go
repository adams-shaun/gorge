package dirac

// firstLiveExtra0 is the first still-legal extra object of target slot 0, or
// NoTarget. RefTargetOwner reads it when target 0 itself went illegal: gorge's
// TargetedOwner is the owners of the legal targets (stack.go resolves with
// targets = legal), so a later target of a multi-target slot (Lodestone
// Bauble's up to four cards) still names its owner.
func (s *State) firstLiveExtra0(k *prep, e *stackEntry) TCode {
	xt := s.xtOf(e)
	for j := 0; j < int(xt.n); j++ {
		if xt.slot[j] == 0 && s.xtLegalRes(k, e, j, 0) {
			return xt.code[j]
		}
	}
	return TCode(NoTarget)
}
