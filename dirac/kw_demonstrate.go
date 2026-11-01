package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Demonstrate (OpDemonstrate): the Demonstrate keyword's cast trigger
// (cards/kw_demonstrate.go) resolves by asking the spell's controller
// whether to copy it, then, on yes, copying it once for that controller and
// once for each living opponent (gorge effects/demonstrate.go). Each copy's
// controller may choose new targets (CR 702.144a), inside this resolution. A
// copy is not cast, so it cannot re-fire.
//
// This kernel is two-player: the single opponent's copy follows the
// caster's with no opponent pick (gorge asks only when several opponents
// could copy, the strict-supersets convention).

// demonstrate runs OpDemonstrate; false: the election is posed.
func (s *State) demonstrate(k *prep, in *kop.Instr) bool {
	if s.spellStackIndex(s.objRefObj(in.Obj)) < 0 {
		return true
	}
	s.res.ask, s.res.askP = raGenDemonstrate, s.res.e.ctrl
	return false
}

// spellStackIndex is the stack index of spell src below the resolving
// entry, or -1 when it is no longer on the stack.
func (s *State) spellStackIndex(src Obj) int {
	if src == NoObj || s.objs[src].zone != kop.ZoneStack {
		return -1
	}
	for i := int(s.sp) - 2; i >= 0; i-- {
		if s.stack[i].kind == seSpell && s.stack[i].obj == src {
			return i
		}
	}
	return -1
}

// demonstrateCopy mints the demonstrate copies after a yes answer: left 2 is
// the caster's and then the living opponent's, 1 the opponent's alone. False:
// a copy's election is posed, or the game stalled.
func (s *State) demonstrateCopy(k *prep, left uint8) bool {
	in := &s.prog(k)[s.res.pc]
	si := s.spellStackIndex(s.objRefObj(in.Obj))
	if si < 0 {
		return true
	}
	caster := s.res.e.ctrl
	alive := !s.pl[caster^1].lost
	if left == 2 {
		if !s.mintSpellCopy(k, si, caster) || s.copyElect(k, b2u(alive)) {
			return false
		}
	}
	return !alive || s.mintSpellCopy(k, si, caster^1) && !s.copyElect(k, 0)
}
