package dirac

// pendDropsAll reports whether every trigger a prefix tap queued on s leaves
// no stack object behind once it is put on the stack: a target-requiring
// trigger with no legal target is removed as it is pushed (gorge's TriggerPush
// is followed at once by its "no legal targets" removal, CR 603.3d), so the
// gorge search's node sees an empty stack where the kernel's queue still
// counts it (C.A.M.P.'s counter trigger on a fortified land when its
// controller has no creature). The probe runs the flush on a fork and reads
// whether anything is still pending or stacked. The queue holds the triggers
// of every tap the path made, but gorge's search places each tap's own
// before the next gap is activated, so the flush reads them tap by tap
// (flushPrefixPending): two fortified lands tapped in turn leave an empty
// stack, not an order choice (C.A.M.P.'s second Fortify).
func (s *State) pendDropsAll(k *prep) bool {
	t := s.fork()
	t.flushPrefixPending(k, true)
	ok := !t.over && t.npend == 0 && t.sp == s.sp && t.ctl == s.ctl
	s.unfork()
	return ok
}
