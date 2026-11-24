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
	if int(pc) >= len(prog) || prog[pc].Op != kop.OpOverride || prog[pc].B.N != kop.OvEnterWith {
		return false
	}
	// a union return (RefTargets012, execUnion) runs a copy of the instruction
	// with only its subject rewritten, once per target
	cur := prog[s.res.pc]
	if unionMask(cur.Obj) != 0 {
		cur.Obj = in.Obj
	}
	return &prog[s.res.pc] == in || cur == *in
}

// enterWithArm arms in's OvEnterWith override for the entrant the move below
// is about to make, when no enters-check-first path (enterAssimilated) took
// it: a return over a target union (Relive the Past) holds every entrant's
// enters checks until the last is back, and the override's own op runs after
// that, so a watcher's power limit (Marketwatch Phantom) read the printed
// 2/1 of a creature that enters as a 5/5 Elemental (CR 614.12).
func (s *State) enterWithArm(k *prep, in *kop.Instr) bool {
	if s.enterWithOverride(k, in) {
		s.g.ewPend = true
		return true
	}
	return false
}

// enterWithSettle applies an armed override enterGen left pending and marks the
// override op that follows spent (once per entrant).
func (s *State) enterWithSettle(k *prep, sl Slot, armed bool) {
	if !armed {
		return
	}
	if s.g.ewPend {
		s.g.ewPend = false
		s.enterWithApply(k, sl)
	}
	s.g.ewDone++
}

// enterAssimilated enters o under ctrl with the override, then runs its enters check.
func (s *State) enterAssimilated(k *prep, o Obj, ctrl uint8, from kop.Zone) Slot {
	var eb entBatch
	// the override applies as the object enters, before the as-enters
	// replacements read it (CR 614.12; gorge applyStaticEffect ahead of the
	// entry move): Valkyrie's Call's returned creature is an Angel to Giada,
	// Font of Hope's extra counters. enterGen spends ewPend.
	s.g.ewPend = true
	sl := s.enterHeld(k, &eb, o, ctrl, false, from)
	if s.g.ewPend {
		s.g.ewPend = false
		s.enterWithApply(k, sl)
	}
	if from == kop.ZoneGraveyard {
		s.fireLeavesGraveyardSelf(k, sl, o)
	}
	s.g.ewDone++ // the override op that follows is spent (once per entrant)
	s.fireEntries(k, &eb)
	return sl
}

// enterWithApply applies the OvEnterWith override that follows the
// resolving return to its entrant at sl.
func (s *State) enterWithApply(k *prep, sl Slot) {
	next := &s.prog(k)[s.res.pc+1]
	s.applyOverrideFl(k, sl, int(next.C), s.res.e.card, s.ovDurFl(next.A.N))
}
