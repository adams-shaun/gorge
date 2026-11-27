package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An until-leaves return after a departure batch (CR 603.6a, 610.3, 610.3d):
// a batch is one simultaneous event, so what its sources exiled comes back once
// its last permanent has gone, as one batch of its own. The departure frames
// set batchHeld; leaveGen records each source (holdReturns) and the outermost frame
// returns their cards at its end (flushReturns), with the enters checks held
// (genState.held, a mask of entry stamps mod 8) until every card is back, then
// fired in entry order.

const (
	batchHeld uint8 = 1 << 6 // a departure frame holds the enters checks of until-leaves returns
)

// tableLinkRet reports whether some program exiles until its source leaves.
func tableLinkRet(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExileUntilLeaves || in.Op == kop.OpChangeZoneAllEx && in.B.N&kop.CzUntilLeaves != 0 || in.Op == kop.OpChooseMove && (in.B.N == 1 || battleLinkPick(t, &in)) {
					return true
				}
			}
		}
	}
	return false
}

// holdRec is an open departure frame's hold: the outer batchHeld bit, whether
// the table holds at all, and the entry stamp at open.
type holdRec struct {
	h  uint8
	on bool
	ts uint16
}

// holdOpen starts a departure frame's hold; the result goes to holdClose.
func (s *State) holdOpen(k *prep) holdRec {
	if k.gen == nil || !k.gen.anyLinkRet {
		return holdRec{}
	}
	return s.holdEntries()
}

// holdEntries opens the frame whatever the table holds: its enters checks wait for
// holdClose (enterMoved).
func (s *State) holdEntries() holdRec {
	r := holdRec{h: s.g.batchLatch & batchHeld, on: true, ts: s.ts}
	if r.h == 0 {
		s.g.held = 0
	}
	s.g.batchLatch |= batchHeld
	return r
}

// holdClose ends the frame: the outermost one fires the held checks. A
// return that parks an ask keeps the frame open across it (resumeReturns):
// gorge holds the whole entry batch until the batch's last entry (CR 603.6a,
// 610.3d), the ask included.
func (s *State) holdClose(k *prep, r holdRec) {
	if !r.on {
		return
	}
	flush := r.h == 0 && s.g.nretHeld != 0
	old := s.g.batchLatch
	parked := false
	if flush {
		s.g.batchLatch &^= batchOpenMask | batchSac
		if k.gen.anyBatch {
			s.batchOpen()
		}
		parked = s.flushReturns(k, false)
	}
	if parked {
		s.g.retHold, s.g.retOld, s.g.retStamp = true, old, r.ts
		return
	}
	s.g.batchLatch = s.g.batchLatch&^batchHeld | r.h
	if r.h == 0 && s.g.held != 0 {
		s.fireHeld(k, r.ts)
	}
	if flush {
		s.batchClose(old)
	}
}

// holdReturns records the source o, leaving slot sl inside an open frame, when
// it has links to return.
//
//go:noinline
func (s *State) holdReturns(sl Slot, o Obj) {
	for i := 0; i < int(s.g.nlinks); i++ {
		if l := &s.g.links[i]; l.src == o && s.objs[o].ver == l.sver+1 {
			if int(s.g.nretHeld) == len(s.g.retHeld) {
				s.capFull(StallLinks) // more linked sources leaving at once than retHeld holds
				return
			}
			s.g.retHeld[s.g.nretHeld] = o
			s.g.retCtrl |= uint16(s.perms[sl].ctrl) << (2 * s.g.nretHeld)
			s.g.nretHeld++
			return
		}
	}
}

// retAskQueuedSince reports that a return queued its own ask (queueAskRet,
// queueRiotRet: an ask with no permanent slot) at ask index nask or later.
func (s *State) retAskQueuedSince(nask uint8) bool {
	for i := int(nask); i < int(s.g.nask); i++ {
		if s.g.ask[i].sl == NoSlot {
			return true
		}
	}
	return false
}

// flushReturns returns the cards of the sources holdReturns recorded. gorge
// suspends its tape while a return's own ask waits for an answer (the parked
// as-enters choice of replacement_etb.go, and the CR 303.4f host question
// asked with the Aura still in exile): the sweep that met the ask stops with
// the asking card in exile, and closeZoneBatch defers the sweep's rest while
// the tape unwinds, so every later return waits too. The kernel mirrors that
// suspension in retHold: a return whose ask parks (preReturn, mid-resolution)
// or queues (queueAskRet, a state-based departure) stops the flush there, the
// rest staying in retHeld for the answer (resumeReturns,
// resumeQueuedReturns), and a plain flush while such an ask still waits is
// refused entirely -- its sources are kept in retHeld behind the pending ask.
// The resumptions pass resume to get past that gate; each flush may meet the
// next asking return and hold behind it again, one answer at a time, in entry
// order.
//
// A QUEUED ask holds the flush only while the flush runs inside a flow gorge
// wraps in a kernel run (holdQ, false in the plain state-based-action pass
// before priority): there the ask goes through the tape path and its answer
// unwinds the sweep (Kernel.Answer's unwind panic, the Ward class --
// measured against gorge, the step-begin flushes hold, the pre-priority ones
// do not). Outside such a flow gorge's ask is an ordinary legacy pose: the
// sweep never stops and every later card returns while the asker waits in
// exile.
//
//go:noinline
func (s *State) flushReturns(k *prep, resume bool) bool {
	n, ctrl := int(s.g.nretHeld), s.g.retCtrl
	if !resume && s.g.retHold {
		// a return ask still waits on its answer: gorge's sweep cannot get
		// past it, so this frame's sources stay held (their holdReturns
		// entries are already in retHeld, in entry order)
		return true
	}
	holdQ := s.ctl == ctlStepBegin
	s.g.nretHeld, s.g.retCtrl = 0, 0
	for i := 0; i < n; i++ {
		was, nask := s.g.pre.state, s.g.nask
		s.returnLinked(k, uint8(ctrl>>uint(2*i)&3), s.g.retHeld[i], resume, holdQ)
		if was == 0 && s.g.nask > nask && s.holdBehindQueuedAsk(i, n, ctrl) {
			return true // zone_untilleavesqask.go
		}
		if was == 0 && s.g.pre.state == preReturn {
			// a return's ask is parked: gorge asks it with the rest of the
			// batch still in exile and returns them once it is answered
			// (resumeReturns)
			if i+1 < n {
				copy(s.g.retHeld[:], s.g.retHeld[i+1:n])
				s.g.retCtrl = ctrl >> uint(2*(i+1))
				s.g.nretHeld = uint8(n - i - 1)
			}
			s.refreshTy(k)
			return true
		}
		if was == 0 && holdQ && s.retAskQueuedSince(nask) {
			// this return queued its own host choice: the card stays in
			// exile behind it and the rest of the batch waits with it (CR
			// 603.6a, 610.3d, 303.4f)
			copy(s.g.retHeld[:], s.g.retHeld[i:n])
			s.g.retCtrl = ctrl >> uint(2*i)
			s.g.nretHeld = uint8(n - i)
			s.refreshTy(k)
			return true
		}
	}
	s.refreshTy(k)
	return false
}

// resumeReturns returns the sources a parked ask held back (flushReturns) once
// it is answered, and closes the departure frame that kept them held. A later
// return that parks again poses its own ask and reports true.
func (s *State) resumeReturns(k *prep) bool {
	if s.g.nretHeld != 0 && s.flushReturns(k, true) {
		if s.g.pre.state != preReturn {
			return false // held behind a queued ask: its answer resumes (answerQueuedHold, resumeQueuedReturns)
		}
		s.res.ask, s.res.askP = raGenRetChoice, s.objs[s.g.pre.obj].owner()
		return true
	}
	if s.g.retHold {
		s.finishHeldFrame(k)
	}
	return false
}

// resumeQueuedReturns flushes the return batch a queued return ask held back
// (flushReturns) once that ask has settled (retAuraAsk, answerQueuedAsk). The
// flush may queue the next returning card's own ask and hold the rest behind
// it again; when nothing is held the departure frame that kept the batch
// closes and its held enters checks fire (finishHeldFrame).
func (s *State) resumeQueuedReturns(k *prep) {
	if s.g.nretHeld == 0 && !s.g.retHold {
		return
	}
	if s.g.nretHeld != 0 && s.flushReturns(k, true) {
		return // another return asked: the rest stays in exile behind it
	}
	if s.g.retHold {
		s.finishHeldFrame(k)
	}
}

// resumeEntryBatch resumes after a return completed an entry batch: it poses
// the next parked return's ask (CR 610.3d) or finishes the frame the leaving
// source opened (leaveGen, turn_gen.go). It runs for a paying return too —
// without it the batch's held enters checks never fire. True means the caller
// stops resolving: the batch's next return asks its host.
func (s *State) resumeEntryBatch(k *prep) bool {
	return s.resumeReturns(k)
}

// finishHeldFrame closes the departure frame a parked return kept open: the
// held enters checks fire together, in entry order, then the batch closes.
func (s *State) finishHeldFrame(k *prep) {
	s.g.retHold = false
	s.g.batchLatch &^= batchHeld
	if s.g.held != 0 {
		s.fireHeld(k, s.g.retStamp)
	}
	// the caller that opened the batch has closed it already (holdClose
	// returned before its own close): restoring retOld would re-open it, so
	// only the batch holdClose itself opened is closed.
	s.g.batchLatch &^= batchOpenMask | batchSac
}

// parkReturnAsk poses the ask of a return a departure frame parked
// (holdClose), once the answer that made the frame has stopped advancing the
// program; true when the ask is live. It must be posed before the next
// instruction, so the held entry batch does not outlive the op that made it
// and the resolution cannot reach its end with the frame still open.
//
//go:noinline
func (s *State) parkReturnAsk() bool {
	if s.g.pre.state != preReturn {
		return false
	}
	s.res.ask, s.res.askP = raGenRetChoice, s.objs[s.g.pre.obj].owner()
	s.ctl = ctlResolve
	return true
}

// finishParkedReturn consumes a return parked outside the program loop
// (leaveGen) as the resolution ends, and completes the departure frame that
// held it (holdClose's parked early return). It reports whether another held
// return parks its own ask: the resolution then stays active until that
// answer finishes the frame.
//
//go:noinline
func (s *State) finishParkedReturn(k *prep) bool {
	if s.g.pre.state != preReturn {
		return false
	}
	o := s.g.pre.obj
	s.g.pre = preAsk{}
	if s.objs[o].zone == kop.ZoneExile {
		from := s.detach(o)
		s.enterLinkRet(k, o, s.objs[o].owner(), from)
	}
	return s.resumeReturns(k)
}

// enterLinkRet is enterFromGrantedZone for an until-leaves return.
func (s *State) enterLinkRet(k *prep, o Obj, ctrl uint8, from kop.Zone) Slot {
	if s.g.batchLatch&batchHeld == 0 {
		return s.enterFromGrantedZone(k, o, ctrl, false, from)
	}
	if k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	sl := s.enterNF(k, o, ctrl, false, from, 0)
	s.g.held |= 1 << (s.perms[sl].ts & 7)
	return sl
}

// enterMoved is enter for a card a move puts onto the battlefield: inside an open frame
// its enters check waits for the others moving with it (CR 603.6a).
func (s *State) enterMoved(k *prep, o Obj, ctrl uint8, tapped bool, from kop.Zone) Slot {
	if s.g.batchLatch&batchHeld == 0 {
		if from == kop.ZoneGraveyard {
			return s.enterFromGrave(k, o, ctrl, tapped)
		}
		return s.enter(k, o, ctrl, tapped, from)
	}
	sl := s.enterNF(k, o, ctrl, tapped, from, 0)
	if from == kop.ZoneGraveyard {
		s.fireLeavesGraveyardSelf(k, sl, o)
	}
	s.g.held |= 1 << (s.perms[sl].ts & 7)
	return sl
}

// fireHeld runs the held enters checks, in entry order. The held entrants
// are the battlefield permanents entered since the frame opened (ts after
// stamp) whose entry stamp's low three bits are set in genState.held; a
// frame that held entrants only through returns, and fewer than eight
// entries, tells them apart exactly.
//
//go:noinline
func (s *State) fireHeld(k *prep, stamp uint16) {
	var eb entBatch
	var ts [MaxPerm]uint16
	hm := s.g.held
	s.g.held = 0
	for m := s.occ; m.any(); {
		sl := m.first()
		m.clr(sl)
		p := &s.perms[sl]
		if p.ts <= stamp || hm&(1<<(p.ts&7)) == 0 {
			continue
		}
		i := eb.n
		for i > 0 && ts[i-1] > p.ts {
			ts[i], eb.sl[i], eb.o[i] = ts[i-1], eb.sl[i-1], eb.o[i-1]
			i--
		}
		ts[i], eb.sl[i], eb.o[i] = p.ts, sl, p.obj
		eb.n++
	}
	s.fireEntries(k, &eb)
}
