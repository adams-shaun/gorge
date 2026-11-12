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
				if in.Op == kop.OpExileUntilLeaves || in.Op == kop.OpChangeZoneAllEx && in.B.N&kop.CzUntilLeaves != 0 || in.Op == kop.OpChooseMove && in.B.N == 1 {
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
		parked = s.flushReturns(k)
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
			s.g.retCtrl |= s.perms[sl].ctrl << s.g.nretHeld
			s.g.nretHeld++
			return
		}
	}
}

// flushReturns returns the cards of the sources holdReturns recorded. When a
// return parks an ask it stops, leaving the rest in retHeld for resumeReturns;
// the caller keeps the entry frame open. A queued ask (retAuraAsk, outside a
// resolution) cannot park this way: every returning Aura queues its own ask and
// the rest of the batch stays in exile until those answers land, in entry order.
//
//go:noinline
func (s *State) flushReturns(k *prep) bool {
	n, ctrl := int(s.g.nretHeld), s.g.retCtrl
	s.g.nretHeld, s.g.retCtrl = 0, 0
	for i := 0; i < n; i++ {
		was := s.g.pre.state
		s.returnLinked(k, ctrl>>i&1, s.g.retHeld[i])
		if was == 0 && s.g.pre.state == preReturn {
			// a return's ask is parked: gorge asks it with the rest of the
			// batch still in exile and returns them once it is answered
			// (resumeReturns)
			if i+1 < n {
				copy(s.g.retHeld[:], s.g.retHeld[i+1:n])
				s.g.retCtrl = ctrl >> uint(i+1)
				s.g.nretHeld = uint8(n - i - 1)
			}
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
	if s.g.nretHeld != 0 && s.flushReturns(k) {
		s.res.ask, s.res.askP = raGenRetChoice, s.objs[s.g.pre.obj].owner()
		return true
	}
	if s.g.retHold {
		s.finishHeldFrame(k)
	}
	return false
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
