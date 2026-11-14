package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// grantKwAll is kop.ExtGrantKwAll: each battlefield permanent matching F
// gains the full keyword mask until end of turn, walked as OpPumpAll walks
// (its keyword arm, without the P/T it does not carry).
//
//go:noinline
func (s *State) grantKwAll(k *prep, in *kop.Instr) {
	e := &s.res.e
	src := s.objRef(k, kop.RefSelf)
	kw := kop.KwMask(uint16(in.C)) | kop.KwMask(uint16(in.B.N))<<16
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPerm(k, in.F, sl, e.ctrl, src) {
			continue
		}
		s.touchSl(sl)
		s.grantEOT(sl, kw)
		s.promoKw(k, sl, kw)
	}
}
