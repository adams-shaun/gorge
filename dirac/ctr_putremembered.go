package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// putCounterRemembered is OpPutCounter with Obj$ Remembered: gorge's
// effPutCounter loops the whole Defined$ set (engine/effects/counters_put.go:240;
// Defined$ Remembered is the resolution's whole remembered set,
// engine/effects/context.go:634), but the kernel's objRef resolves one
// object (refObjCtx returns rem[0]). CounterNum$ is read once, before the
// loop: gorge computes n before its recipient walk (only
// CounterNumPerDefined$ re-reads per object, and the kernel does not lower
// that).
//
//go:noinline
func (s *State) putCounterRemembered(k *prep, in *kop.Instr) {
	n := s.value(k, in.A)
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		sl := s.objs[o].slot
		m := s.placeCounters(k, sl, int(in.C), n, s.perms[sl].ctrl, ctrPutEffect)
		if m > 0 && k.anyCtrAdded {
			s.ctrNote(k, sl, int(in.C))
		}
		if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
		}
	}
}

// doubleCountersRemembered is OpDoubleCounters with Obj$ Remembered: gorge's
// effMultiplyCounter doubles each object of the whole Defined$ set
// (engine/effects/counters.go:46), but the kernel's objRef resolves one
// object (refObjCtx returns rem[0]). The overload each-variant is the live
// carrier: Fangs of Kalonia remembers every creature its PutCounterAll
// countered, then doubles each one.
//
//go:noinline
func (s *State) doubleCountersRemembered(k *prep, in *kop.Instr) {
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		s.doubleCountersAt(k, s.objs[o].slot, in.C)
	}
}
