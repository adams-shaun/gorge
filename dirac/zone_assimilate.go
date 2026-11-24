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
	return &prog[s.res.pc] == in || s.unionCopy(k, in)
}

// unionCopy: in is execUnion's copy of the current instruction, its subject
// swapped for one target of the union (Relive the Past's three target slots).
func (s *State) unionCopy(k *prep, in *kop.Instr) bool {
	prog := s.prog(k)
	if int(s.res.pc) >= len(prog) || &prog[s.res.pc] == in {
		return false
	}
	cur := prog[s.res.pc]
	cur.Obj = in.Obj
	return cur == *in
}

// enterAssimilatedHeld is enterAssimilated inside an open entry frame (a union
// return's, execUnion): the override still applies as the object enters, and
// the enters checks wait for the rest of the batch (CR 603.6a; holdClose).
func (s *State) enterAssimilatedHeld(k *prep, o Obj, ctrl uint8, from kop.Zone) Slot {
	s.g.ewPend = true
	sl := s.enterMoved(k, o, ctrl, false, from)
	if s.g.ewPend {
		s.g.ewPend = false
		s.enterWithApply(k, sl)
	}
	s.g.ewDone++ // the override op that follows is spent
	return sl
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
	s.g.ewDone++ // the override op that follows is spent
	s.fireEntries(k, &eb)
	return sl
}

// enterWithApply applies the OvEnterWith override that follows the
// resolving return to its entrant at sl.
func (s *State) enterWithApply(k *prep, sl Slot) {
	next := &s.prog(k)[s.res.pc+1]
	s.applyOverrideFl(k, sl, int(next.C), s.res.e.card, s.ovDurFl(next.A.N))
}
