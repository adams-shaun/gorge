package dirac

// discardPicks moves the answered discards picks in answer order. Gorge asks
// the CR 616.1 order inside the first competing card's move, so that card and
// every later pick wait in the queue, competing or not: at the ask a later
// answered card is still in hand. A discard parked and not yet answered counts
// as an earlier competing pick (picks arriving one at a time, as a cleanup
// discard's do). x is the parked moves' mark (parkCleanup, or 0).
func (s *State) discardPicks(k *prep, picks []Obj, x uint8) {
	first := 0
	if !s.parkedDiscardWaiting() {
		first = len(picks) // none competes: every pick moves now
		for i, o := range picks {
			if s.graveMoveCompetes(k, o) {
				first = i
				break
			}
		}
	}
	for _, o := range picks[:first] {
		s.discardObj(k, o)
	}
	for _, o := range picks[first:] {
		if s.park(k, o, parkDiscard, x) {
			continue
		}
		if !s.parkDiscardQuiet(o, x) {
			s.capFull(StallReplCands)
			break
		}
	}
}

// parkedDiscardWaiting reports a parked discard whose ask is still to come.
func (s *State) parkedDiscardWaiting() bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkDiscard {
			return true
		}
	}
	return false
}
