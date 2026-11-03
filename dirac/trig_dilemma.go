package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Facing a dilemma: a player who chooses one or more modes for a spell or ability
// faces one dilemma, however many modes they chose (gorge events.DilemmaFaced on
// the ModeChosen marker of a Charm's choice). A reversed cast takes it back with
// the triggers it queued (CR 733.1; gorge abortCast, pendingCast.proposalTriggers).
// genPrep.anyDilemma gates every path: some table reads the count or the trigger.

// faceDilemma records one dilemma for p and fires "whenever you face a dilemma".
//
//go:noinline
func (s *State) faceDilemma(k *prep, p uint8) {
	if s.g.dilemmas[p&1] == 255 {
		s.capFull(StallTables) // loud: the byte holds 255
		return
	}
	s.g.dilemmas[p&1]++
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigFacesDilemma }, func(l trigListener, a *kop.Ability) {
			s.playerTrigGen(k, kop.TrigFacesDilemma, l, a, p, 0)
		})
	}
}

// buildModesChosen is the announcement's choice of modes ending: a spell's or an
// activated ability's, whose dilemma a reversal takes back, or a trigger's as it
// goes on the stack.
func (s *State) buildModesChosen(k *prep, a *kop.Ability) {
	b := s.bld()
	if b.e.mode&(1<<a.NModes-1) == 0 {
		return
	}
	lo := s.npend
	s.faceDilemma(k, b.e.ctrl)
	if b.e.kind != seTrigger {
		b.bfl |= bflDilemma
		b.dilLo, b.dilHi = lo, s.npend
	}
}

// unfaceDilemma takes back the dilemma of the build a reversal ends.
func (s *State) unfaceDilemma() {
	b := s.bld()
	s.g.dilemmas[b.e.ctrl&1]--
	copy(s.pend[b.dilLo:s.npend], s.pend[b.dilHi:s.npend])
	s.npend -= b.dilHi - b.dilLo
}
