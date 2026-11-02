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
	r := holdRec{h: s.g.batchLatch & batchHeld, on: true, ts: s.ts}
	if r.h == 0 {
		s.g.held = 0
	}
	s.g.batchLatch |= batchHeld
	return r
}

// holdClose ends the frame: the outermost one fires the held checks.
func (s *State) holdClose(k *prep, r holdRec) {
	if !r.on {
		return
	}
	flush := r.h == 0 && s.g.nretHeld != 0
	old := s.g.batchLatch
	if flush {
		s.g.batchLatch &^= batchOpenMask | batchSac
		if k.gen.anyBatch {
			s.batchOpen()
		}
		s.flushReturns(k)
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
			s.g.retHeld[s.g.nretHeld] = o
			s.g.retCtrl |= s.perms[sl].ctrl << s.g.nretHeld
			s.g.nretHeld++
			return
		}
	}
}

// flushReturns returns the cards of the sources holdReturns recorded.
//
//go:noinline
func (s *State) flushReturns(k *prep) {
	n, ctrl := int(s.g.nretHeld), s.g.retCtrl
	s.g.nretHeld, s.g.retCtrl = 0, 0
	for i := 0; i < n; i++ {
		s.returnLinked(k, ctrl>>i&1, s.g.retHeld[i])
	}
	s.refreshTy(k)
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
