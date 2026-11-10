package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// rememberAll is kop.ExtRememberAll (gorge effPumpAll's RememberPumped$):
// each battlefield permanent matching F joins the remembered set in gorge's
// walk order (seat by seat, battlefield order). A full set stalls
// (remember's StallRemember).
//
//go:noinline
func (s *State) rememberAll(k *prep, in *kop.Instr) {
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		if sl := m.first(); s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) {
			hit = hit.with(sl)
		}
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(hit, &ord)
	for _, sl := range ord[:n] {
		s.remember(s.perms[sl].obj)
	}
}
