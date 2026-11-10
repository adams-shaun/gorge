package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// returnsRemembered reports whether in is a return over a remembered set of
// several objects, which every other read of the ref resolves to the first.
func (s *State) returnsRemembered(in *kop.Instr) bool {
	return in.Obj == kop.RefRemembered && s.g.nrem > 1 && (in.Op == kop.OpReturn || in.C>>3&kop.MvAlsoSelf == 0)
}

// returnRemembered moves every remembered object, in remembered order, as one
// entering event (CR 603.6a): the enters checks wait until all are back, so an
// entrant sees the others come in with it. It reports a pending ask: the object
// it is for stays current in s.g.retAt, and the answer's continueRes re-enters
// the walk, where the moved object is a miss. An entrant's queued as-enters
// election suspends the walk the same way, before the next object moves.
//
//go:noinline
func (s *State) returnRemembered(k *prep, in *kop.Instr) bool {
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	if s.g.retAt == 0 {
		s.g.retAt, s.g.retTS, s.g.held = 1, s.ts, 0
	}
	for int(s.g.retAt) <= int(s.g.nrem) {
		o := s.g.rem[s.g.retAt-1]
		n0 := s.g.nask
		if s.execReturnObj(k, in, NoSlot, o) {
			return true
		}
		if s.g.retAt <= s.g.nrem && s.g.rem[s.g.retAt-1] == o { // else ForgetChanged$ closed the gap
			s.g.retAt++
		}
		if int(s.g.retAt) <= int(s.g.nrem) && s.eachReturnParks(k, in, n0) {
			// the entrant's as-enters election (a reveal or shock land) is
			// asked inside its own move: the objects after it are still in
			// their zone while it is answered, as gorge moves them one at a time
			return true
		}
	}
	s.g.retAt = 0
	if s.g.held != 0 {
		s.fireHeld(k, s.g.retTS)
	}
	return false
}

// returningObj is the object a return instruction is moving.
func (s *State) returningObj(in *kop.Instr) Obj {
	if s.g.retAt != 0 {
		return s.g.rem[s.g.retAt-1]
	}
	return s.objRefObj(in.Obj)
}

// returnLeftOrigin reports whether an OpReturn's object is no longer in a zone its
// Origin$ names: Forsaken Miner's trigger finds nothing once the card has gone to the
// library in response.
func returnLeftOrigin(in *kop.Instr, ob *object) bool {
	return in.B.N != 0 && in.B.N&(1<<ob.zone) == 0
}
