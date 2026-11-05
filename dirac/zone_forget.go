package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugForgetChangedOff is the mutation: with it set,
// forgetChanged forgets nothing (TestForgetChangedMutation).
var DebugForgetChangedOff bool

// forgetChanged is ForgetChanged$ True on one moved object (gorge
// eventForgetChanged / forgetRememberedOne): o leaves the resolution's
// remembered set, order kept. gorge calls it after the object moved, for
// each moved object, so a caller reaches it only past a successful move.
//
//go:noinline
func (s *State) forgetChanged(o Obj) {
	if DebugForgetChangedOff {
		return
	}
	s.forgetRemGen(o)
}

// forgetFound is forgetChanged for a found card of the search at the
// resolving pc, when that search carries SrForgetChanged (Instr.B bit 1).
//
//go:noinline
func (s *State) forgetFound(k *prep, o Obj) {
	if p := s.prog(k); int(s.res.pc) < len(p) && p[s.res.pc].B.N&kop.SrForgetChanged != 0 {
		s.forgetChanged(o)
	}
}

// imprintFound is the chosen-set append of a library hidden pick that carries
// SrImprint (ChangeZone Imprint$ True): the card that actually moved joins
// the chosen set, deduped (gorge zone_change's imprint append).
//
//go:noinline
func (s *State) imprintFound(k *prep, o Obj) {
	if p := s.prog(k); int(s.res.pc) < len(p) && p[s.res.pc].Op == kop.OpChooseMove && p[s.res.pc].B.N&kop.SrImprint != 0 {
		s.imprintChosen(o)
	}
}

// isSearchOp reports whether the resolving pick is a real library search
// (OpSearchEx); an OpChooseMove from the library is a hidden pick, which
// does not fire "whenever you search your library". Outside a program
// (kernel tests) it is a search.
func (s *State) isSearchOp(k *prep) bool {
	p := s.prog(k)
	return !s.res.active || int(s.res.pc) >= len(p) || p[s.res.pc].Op == kop.OpSearchEx
}
