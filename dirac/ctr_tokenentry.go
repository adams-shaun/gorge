package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Token entry counters (CR 122.6, 614.1c): a Token's WithCountersType$ /
// WithCountersAmount$ rider arms genState.enterCtr for the mint that follows
// (ExtTokenCounters), exactly as a ReturnEx carries its entry counters, and
// clears it after. The token enters with them: its own enters triggers and
// the counters' CounterAddedOnce watchers see them.

// tokenCountersExt is ExtTokenCounters.
//
//kmanifest:ext ExtTokenCounters
func (s *State) tokenCountersExt(k *prep, in *kop.Instr) (ok, done bool) {
	n := s.value(k, in.B)
	if n <= 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind, s.g.enterCtr.kind2 = 0, 0, 0
		return true, true
	}
	if n > 255 {
		n = 255
	}
	s.g.enterCtr.n, s.g.enterCtr.kind, s.g.enterCtr.kind2 = uint8(n), kop.CounterKind(in.C), 0
	return true, true
}
