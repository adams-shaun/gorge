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

// DebugHandPickNoImprint is TestEurekaMutation's switch: a hand pick's
// Imprint$ adds nothing. Never set outside a test.
var DebugHandPickNoImprint bool

// imprintFound is the imprint append of a hidden pick that carries SrImprint
// (ChangeZone Imprint$ True). A library pick's moved card joins the chosen
// set, deduped (the DigUntil ImprintFound$ shape, lower_digimprint.go); a
// hand pick onto the battlefield (Eureka) adds the card to the resolution's
// imprinted set when it is a non-token now in zone z (gorge
// settleChangeZoneMoveAs confirms the post-move zone).
//
//go:noinline
func (s *State) imprintFound(k *prep, o Obj, z kop.Zone) {
	if p := s.prog(k); int(s.res.pc) < len(p) && p[s.res.pc].Op == kop.OpChooseMove && p[s.res.pc].B.N&kop.SrImprint != 0 {
		if s.searchZone(k) != kop.ZoneHand {
			s.imprintChosen(o)
			return
		}
		if ob := &s.objs[o]; ob.zone == z && ob.of&ofToken == 0 && !DebugHandPickNoImprint {
			s.imprint(o)
		}
	}
}
