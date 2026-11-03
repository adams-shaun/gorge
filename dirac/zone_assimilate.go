package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Assimilate (gorge effects/assimilate.go): a creature card enters under your control
// with a +1/+1 counter as a Borg artifact creature that loses its other creature
// types. kgen lowers it to OpReturn then an OvEnterWith OpOverride, and the
// override applies ahead of the enters triggers, so a "whenever an artifact enters"
// sees one.

// enterWithOverride: the instruction after return in is its OvEnterWith override.
func (s *State) enterWithOverride(k *prep, in *kop.Instr) bool {
	prog := s.prog(k)
	pc := s.res.pc + 1
	return int(pc) < len(prog) && &prog[s.res.pc] == in && prog[pc].Op == kop.OpOverride && prog[pc].B.N == kop.OvEnterWith
}

// enterAssimilated enters o under ctrl with the override, then runs its enters check.
func (s *State) enterAssimilated(k *prep, o Obj, ctrl uint8, from kop.Zone) Slot {
	var eb entBatch
	sl := s.enterHeld(k, &eb, o, ctrl, false, from)
	if from == kop.ZoneGraveyard {
		s.fireLeavesGraveyardSelf(k, sl, o)
	}
	next := &s.prog(k)[s.res.pc+1]
	s.applyOverrideFl(k, sl, int(next.C), s.res.e.card, s.ovDurFl(next.A.N))
	s.fireEntries(k, &eb)
	return sl
}
