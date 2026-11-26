package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a free cast is announced before the rest of the
// program that offered it. gorge's effDiscover runs effPlay, and the tail
// ("if you don't cast it, put it into your hand", Hit the Mother Lode's
// Treasures) only when the play is not suspended (effects/discover.go):
// runPlays begins the picked casts, the target ask included, and the parked
// resolution resumes after the last one is made (rules play_queue.go).
//
// The kernel makes the free casts at priority, after the resolution
// (genState.freeQ, turn_gen.go stepGen, the spell hold of trig_xpaid.go).
// So that the tail still runs after them, the close of the OpCastFree
// election parks the program: the resolving entry leaves the stack as a
// finished resolution's does, and res.ask = raGenFreeParked marks the
// program waiting at res.pc. stepGen makes the casts through the existing
// hold, and when the queue is empty (before the casts' triggers are placed
// or any state-based action runs) resumeFreeCast puts the resolving entry
// back on the stack and runs the rest of the program, whose finishResolve
// pops it as it always does. No State byte is added: the marker is a value of
// res.ask, which is read only under ctlResolve and res.active.
const raGenFreeParked = raGenManifestDread + 70

// DebugNoFreeCastPark restores the old order (the tail runs before the
// casts): the mutation test.
var DebugNoFreeCastPark bool

// castFreeClosed is the close of the OpCastFree election: every path of
// raGenCastFree ends here.
func (s *State) castFreeClosed(k *prep) {
	if s.g.nfq == 0 || DebugNoFreeCastPark || !s.freeCastParkable() {
		s.continueRes(k, 0)
		return
	}
	s.res.ask = raGenFreeParked
	s.sp-- // the resolving entry leaves the stack (finishResolve's first step)
	s.res.active = false
	s.afterResolve()
}

// freeCastParkable: nothing the program holds is lost to the casts. The
// build overlays the resolution's decision scratch (look, dmgSeqOrd:
// scratch.go), and a queued as-enters ask is served by the
// program's own continuation (midResAsk); either keeps the old order.
func (s *State) freeCastParkable() bool {
	return s.g.nlook == 0 && !s.g.dmgSeqOn && s.g.nask == 0
}

// resumeFreeCast runs the rest of a parked program once its casts are
// announced: the resolving entry goes back where the hold push put it --
// under the casts (gorge's suspended resolution object never left the
// stack, and resumes from there while the spells cast from it sit above,
// CR 608.2g; the tail's asks read it there, and finishResolve removes it
// there through the still-set hold marker) -- or, when no hold was taken,
// on top of the stack; then the program continues after the OpCastFree.
func (s *State) resumeFreeCast(k *prep) {
	s.res.ask = raNone
	// the drained queue's register has served its casts (a committed card
	// was dropped in cast.go finishBuild; a refused or reversed one kept its
	// place). The whole register dies with the queue: ReplaceGraveyard$
	// Exile belongs to the played spells, and a stale bit would exile a
	// later unrelated free cast's spell on leave (CR 701.5a). This runs
	// before turn_gen.go's drain tail on the parked path.
	s.g.freeP = 0
	if s.holdIndex() < 0 {
		// the hold ends here, as dropHold ends it: nothing is left to
		// restore on a later stepGen (a pass taken before it would be
		// restored over). With a held copy in place the hold stays set until
		// finishResolve takes the entry off at its position.
		s.g.hold = false
		if int(s.sp) >= MaxStack {
			s.capFull(StallFree)
			return
		}
		s.push(s.res.e)
	}
	if s.g.priSave != 0 {
		s.prio, s.g.priSave = SeatIx(s.g.priSave), 0
	}
	s.res.active = true
	s.continueRes(k, 0)
}

// holdIndex: the stack index of the held resolved entry, or -1.
func (s *State) holdIndex() int {
	spell := s.res.e.kind == seSpell
	for i := 0; i < int(s.sp); i++ {
		if e := &s.stack[i]; spell && s.spellHeld(e) || !spell && e.flags&sfHold != 0 && e.kind != seSpell {
			return i
		}
	}
	return -1
}

// freeBegun: o is a remembered card whose free cast the kernel has begun
// (it is on the stack under sfFree). A tail that moves "the remembered
// card" (an "if you don't cast it" bounce) finds a spell, not a card in its
// zone, as freeQueued says of one still waiting.
func (s *State) freeBegun(o Obj) bool {
	if s.objs[o].zone != kop.ZoneStack {
		return false
	}
	i := s.stackIndexOf(o)
	if i < 0 || s.stack[i].flags&sfFree == 0 {
		return false
	}
	for _, r := range s.g.rem[:s.g.nrem] {
		if r == o {
			return true
		}
	}
	return false
}
