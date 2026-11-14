package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An effect's sacrifice batch (flushSac) whose move two or more exile-instead
// replacements compete for parks that move for the CR 616.1 order ask: gorge
// poses the ask from the emitting primitive and emits the batch's later
// sacrifices only after the answer, so they wait in the queue behind it and
// massMovesGoOn runs (and re-asks) them one at a time. Each parked move is
// sacrificed as it moves (parkSacd: its Sacrificed triggers and tallies come
// after the answer, as gorge's emit does), and gorge's departure batch stays
// open across the asks (BatchDepartures over the chosen list), so the kernel
// keeps its batch latch open too and the last parked move closes it
// (parkBatchEnd): a "one or more ... leave/die/are sacrificed" line queues
// once for the whole flush.
//
// A parked flush whose departures hold an until-leaves return (holdOpen) stalls
// by name: gorge returns after the whole flush, which the parked moves cannot
// reproduce once the frame has closed (flushSac).

// sacReturnHold reports whether an until-leaves return waits where a sacrifice
// would park its CR 616.1 ask (sacDelayPark): a departed permanent's cards wait
// to come back (nretHeld), the returns batch is already parked on an ask
// (retHold), or the permanent to sacrifice holds a link to return — gorge
// returns after the whole departure batch, which the parked move cannot
// reproduce once the frame has closed (zone_untilleavesbatch.go). The
// sacrifice then happens now and dieZone stalls by name, as before.
func (s *State) sacReturnHold(ord []Slot) bool {
	if s.g.nretHeld != 0 || s.g.retHold {
		return true
	}
	for _, sl := range ord {
		o := s.perms[sl].obj
		for i := 0; i < int(s.g.nlinks); i++ {
			if l := &s.g.links[i]; l.src == o && s.objs[o].ver == l.sver+1 {
				return true
			}
		}
	}
	return false
}

// sacDelayPark parks a delayed trigger's sacrifice (resolveDelayed's
// OpSacrifice, an evoke's or an AtEOT mover's) when its move to the graveyard
// is one two or more exile-instead replacements compete for (CR 616.1): gorge
// leaves the permanent where it is, asks its controller for the order, and
// makes the sacrifice with the move, after the answer (runParkedMass). false
// parks nothing: the move does not compete, the move already waits on an ask,
// the departures hold an until-leaves return the parked move cannot place
// (sacReturnHold), or the queue cannot hold it — the move happens now and
// dieZone stalls by name, as before.
func (s *State) sacDelayPark(k *prep, sl Slot) bool {
	o := s.perms[sl].obj
	if !s.graveMoveCompetes(k, o) || s.parkedWaitingOn(o) {
		return false
	}
	var ord [1]Slot
	ord[0] = sl
	if s.sacReturnHold(ord[:]) {
		return false
	}
	if !s.park(k, o, parkMass, 0) {
		return false
	}
	if r := s.parkedMassOf(o); r != nil {
		r.fl |= parkSacd
	}
	return true
}

// sacParkFrom parks the first move of ord that competes and returns its
// index (len(ord) and false when none does). A competing move the queue
// cannot hold stalls by name.
func (s *State) sacParkFrom(k *prep, ord []Slot) (int, bool) {
	if g := k.gen; g == nil || g.anyRepl&rbExileOnLeave == 0 && g.ix[mechExileEx] == arenaNone {
		return len(ord), false // no exile-instead effect: nothing competes (graveMoveCompetes)
	}
	for w, sl := range ord {
		o := s.perms[sl].obj
		if s.park(k, o, parkMass, 0) {
			return w, true
		}
		if s.graveMoveCompetes(k, o) {
			s.capFull(StallReplLeave) // more moves or sources than the parked queue holds
			return w, false
		}
	}
	return len(ord), false
}

// sacParkTail marks the parked head of ord a sacrifice, queues the rest
// behind it, and with a batch open hands its close to the last of them;
// true when it did (the caller leaves the batch open).
func (s *State) sacParkTail(ord []Slot, batch bool, bOld uint8) bool {
	last := s.parkedMassOf(s.perms[ord[0]].obj)
	if last == nil {
		s.capFull(StallReplLeave) // the head was parked as a waiting ask: the rest cannot queue behind it
		return false
	}
	last.fl |= parkSacd
	for _, sl := range ord[1:] {
		r := s.parkMassQuiet(s.perms[sl].obj)
		if r == nil {
			s.capFull(StallReplCands) // the park queue is full: a declared limit
			break
		}
		r.fl |= parkSacd
		last = r
	}
	if !batch {
		return false
	}
	last.fl |= parkBatchEnd
	last.x = bOld
	return true
}

// parkedMassOf is o's queued mass move (nil: none).
func (s *State) parkedMassOf(o Obj) *parkedMove {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkMass && r.obj == o {
			return r
		}
	}
	return nil
}

// runParkedMass runs a parked mass move: a sacrifice is emitted first, and
// the batch's last move closes the batch its flush left open; a destroyed
// RememberDestroyed$ victim is remembered once it reaches the graveyard.
func (s *State) runParkedMass(k *prep, r parkedMove) {
	if ob := &s.objs[r.obj]; r.fl&parkSacd != 0 && ob.zone == kop.ZoneBattlefield {
		s.fireSac(k, ob.slot)
	}
	s.leave(k, s.objs[r.obj].slot, kop.ZoneGraveyard, false)
	switch {
	case r.fl&parkSacd != 0:
		if r.fl&parkBatchEnd != 0 {
			s.batchClose(r.x)
		}
	case r.fl&parkRemember != 0 && s.objs[r.obj].zone == kop.ZoneGraveyard:
		s.remember(r.obj) // RememberDestroyed$: only a victim that reached the graveyard counts
	}
}
