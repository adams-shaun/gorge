package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
				*c = uint8(min(int(*c)*2, 255))
			}
		}
	}
}
