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

// sacParkFrom parks the first move of ord that competes and returns its
// index (len(ord) and false when none does). A competing move the queue
// cannot hold stalls by name.
func (s *State) sacParkFrom(k *prep, ord []Slot) (int, bool) {
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
// the batch's last move closes the batch its flush left open.
func (s *State) runParkedMass(k *prep, r parkedMove) {
	if ob := &s.objs[r.obj]; r.fl&parkSacd != 0 && ob.zone == kop.ZoneBattlefield {
		s.fireSac(k, ob.slot)
	}
	s.leave(k, s.objs[r.obj].slot, kop.ZoneGraveyard, false)
	if r.fl&parkBatchEnd != 0 {
		s.batchClose(r.x)
	}
}
