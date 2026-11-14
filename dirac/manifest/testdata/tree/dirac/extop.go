package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// execExt is the fixture's hand switch.
func (s *State) execExt(k *prep, in *kop.Instr) (ok, done bool) {
	switch kop.ExtOp(in.A.N) {
	case kop.ExtLegacy:
		return true, true
	default:
		return s.execExtGen(k, in)
	}
}
