package dirac

// An until-leaves batch whose returning card queues an as-enters ask (Outpost
// Siege's Khans-or-Dragons choice, a GenericChoice enters body): gorge poses
// the ask at that card's entry, mid-batch, and the returns after it wait for
// the answer (CR 610.3d; the entry walker suspends the sweep's resolution), so
// the cards behind it are still in exile when the ask is posed. The kernel
// returned the whole batch first and posed the queued ask after it. The
// remaining returns are held in retHeld, as a parked pre-entry ask holds them
// (flushReturns), and the queued ask's answer resumes them
// (answerQueuedHold). DebugReturnQueuedAskNoHold is the lockstep mutation.

// DebugReturnQueuedAskNoHold returns the whole batch before a queued
// as-enters ask is posed, the old order.
var DebugReturnQueuedAskNoHold bool

// holdBehindQueuedAsk keeps the returns after source i's queued ask (of n held
// sources, controllers ctrl) in retHeld: the rest of source i's own links
// (returnLinked stops after the entry that queued the ask), then the sources
// after it. It reports whether it did: the batch stops there.
//
//go:noinline
func (s *State) holdBehindQueuedAsk(i, n int, ctrl uint16) bool {
	if DebugReturnQueuedAskNoHold || !s.res.active {
		return false
	}
	from := i + 1
	if s.hasLinks(s.g.retHeld[i]) {
		from = i
	}
	if from >= n {
		return false
	}
	copy(s.g.retHeld[:], s.g.retHeld[from:n])
	s.g.retCtrl = ctrl >> uint(2*from)
	s.g.nretHeld = uint8(n - from)
	return true
}

// hasLinks reports whether source o still has an until-leaves link to return.
func (s *State) hasLinks(o Obj) bool {
	for i := 0; i < int(s.g.nlinks); i++ {
		if l := &s.g.links[i]; l.src == o && s.objs[o].ver == l.sver+1 {
			return true
		}
	}
	return false
}

// answerQueuedHold resumes the returns held behind a queued as-enters ask once
// its answer has landed and no other ask of the batch is outstanding. It
// reports whether a return parked its own pre-entry ask (the caller stops).
//
//go:noinline
func (s *State) answerQueuedHold(k *prep) bool {
	if s.g.nretHeld == 0 || s.g.nask != 0 || s.g.pre.state != 0 || !s.res.active {
		return false
	}
	return s.resumeReturns(k)
}
