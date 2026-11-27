package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// doubleCountersOp is OpDoubleCounters: the remembered objects, one object, or
// (Obj You) the controller's player counters.
//
//go:noinline
func (s *State) doubleCountersOp(k *prep, in *kop.Instr) {
	if in.Obj == kop.RefRemembered && s.g.nrem > 1 {
		s.doubleCountersRemembered(k, in)
	} else if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.doubleCountersAt(k, sl, in.C)
	} else if in.Obj == kop.RefController {
		s.doublePlayerCounters(k, in)
	}
}

// doublePlayerCounters is OpDoubleCounters on the controller (Defined$ You,
// every kind): gorge's effMultiplyCounter adds each kind the player holds
// once more, as a raw PlayerCounterChange (no counter replacement runs, so
// plusPlayerCtr is not consulted). The kernel keeps two player counters, energy
// and experience; both saturate at 255 like gainEnergy. A named kind other than
// every kind does nothing here: no lowering produces one.
//
//go:noinline
func (s *State) doublePlayerCounters(k *prep, in *kop.Instr) {
	if in.C != 255 {
		return
	}
	for _, kind := range [...]int{kop.PlCtrEnergy, kop.PlCtrExperience} {
		q := s.players(k, in.Obj)
		for i := uint8(0); i < q.n; i++ {
			if c := s.plCtrPtr(q.s[i], kind); c != nil {
				was := int(*c)
				*c = uint8(min(was*2, 255))
				s.firePlayerCtr(k, q.s[i], kind, was) // the doubling adds as many as it had (trig_playerctr.go)
			}
		}
	}
}
