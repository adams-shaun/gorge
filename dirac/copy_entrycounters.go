package dirac

// copyEnteredCounters is OpBecomeCopy's BecomeCopyCtr rider (gorge's
// RememberCloneOrigin$ True + PutCounter EachFromSource$ Remembered: Dominion
// Saboteur). It puts on the entering permanent at sl one counter of each kind
// the chosen copy origin at src holds, read once before any is placed (CR
// 707.9b/614.12: the copy enters with them, so its enters triggers and the
// state-based actions see them). A counter kind above the inline slots comes
// from the overflow table. The placements are OpPutCountersFrom's
// (putCounterSnapshot: the AddCounter replacements, CounterAddedOnce, and the
// StallCtrOrder guard for two heard kinds), as the rider's PutCounter is in
// gorge.
func (s *State) copyEnteredCounters(k *prep, sl, src Slot) {
	p := &s.perms[src]
	var over [maxCtrOver]ctrOverEntry
	nover := 0
	if p.ck[ctrSlots-1] != 0 {
		nover = copy(over[:], s.ctrOverOf(p.obj, 0))
	}
	s.putCounterSnapshot(k, sl, int(p.p1), p.ck, p.cn, over[:nover], 1)
}
