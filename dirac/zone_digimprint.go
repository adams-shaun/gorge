package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// digImprintMarked reports whether the Dig being resolved is followed by the
// kop.ExtDigImprint marker (Dig Imprint$ True on a permanent's ability): the
// cards it takes join the resolution's imprinted set (zone_imprint.go). The
// program counter stays on the Dig while a take is parked and resumed, the
// way imprintFound reads its own instruction.
func (s *State) digImprintMarked(k *prep) bool {
	p := s.prog(k)
	pc := int(s.res.pc) + 1
	if pc >= len(p) {
		return false
	}
	return p[pc].Op == kop.OpExt && kop.ExtOp(p[pc].A.N) == kop.ExtDigImprint
}
