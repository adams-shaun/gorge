package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// OpAskDestAlt (emeria-shepherd-destzone): ChangeZone's conditional alternate
// destination without MANDATORY. gorge's changeZoneAltDestination
// (engine/effects/zone_change.go) asks the resolving controller, once the
// DestAltSVar$ condition holds and before any move, which of the two zones
// the card takes (askDestinationChoice: changezone_dest_alt, options "primary"
// then "alternate"); the kernel poses the same ChGenDestAlt ask the library
// search's SrDestAltGrave uses (zone_destalt.go). kgen lowers the node as
//
//	SkipIf cond (over the ask)  ;  OpAskDestAlt (A.N = the primary arm's length + 1)
//	PRIMARY...  ;  SkipIf never (over ALT)  ;  ALT...
//
// so the answer 0 (primary) falls into PRIMARY and the answer 1 skips it and
// the jump, landing on ALT.

// execAskDestAlt runs OpAskDestAlt (execOpGen): the ask is always posed, the
// condition guard is the SkipIf before it.
//
//kmanifest:op OpAskDestAlt
func (s *State) execAskDestAlt(k *prep, in *kop.Instr) (ok, done bool) {
	if in.Op != kop.OpAskDestAlt {
		return true, false
	}
	s.res.ask, s.res.askP = raGenDestAlt, s.res.e.ctrl
	return false, true
}

// askDestAltSkip is the answered op's alternate skip: the instructions the
// program jumps over when the controller chose the alternate destination;
// ok false when the parked instruction is not an OpAskDestAlt (a library
// search's SrDestAltGrave ask).
func (s *State) askDestAltSkip(k *prep, a Action) (n uint8, ok bool) {
	prog := s.prog(k)
	if int(s.res.pc) >= len(prog) || prog[s.res.pc].Op != kop.OpAskDestAlt {
		return 0, false
	}
	if a.Mode() == 1 {
		n = uint8(prog[s.res.pc].A.N)
	}
	return n, true
}
