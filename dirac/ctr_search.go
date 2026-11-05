package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugSearchNoCounters is TestPicardMutation's switch: set, the entry
// counters a library search's WithCountersType$/WithCountersAmount$ rider
// arms are dropped, as if the search path ignored the rider.
var DebugSearchNoCounters bool

// searchCounters arms (B > 0) or clears (B <= 0) the entry counters a
// library search's found card enters with. kgen emits it around the
// OpSearchEx for a WithCountersType$/WithCountersAmount$ rider; the entering
// card consumes genState.enterCtr in enter, exactly as a graveyard pick's
// rider does (zone_handgravepick.go handGraveArm).
func (s *State) searchCounters(k *prep, in *kop.Instr) {
	if DebugSearchNoCounters {
		s.g.enterCtr.n = 0
		return
	}
	if n := s.value(k, in.B); n > 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind = uint8(n), kop.CounterKind(in.C)
		return
	}
	s.g.enterCtr.n = 0
}
