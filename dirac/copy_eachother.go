package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Hive Mind's "each other player copies that spell" (gorge copyControllersFor's
// NonTriggeredCardController): OpCopySpell with C&4 gives one copy to each
// living player except W's (the caster), in turn order from the resolving
// controller (gorge AliveFrom), each copy's controller choosing its new
// targets with C&1 (CR 707.10, 707.10c). Cold: only C&4 copies reach it.

// copyEachStart is copyEachOther's left on the first pass: every copy is
// still owed.
const copyEachStart = 255

// copyEachOther makes the copies still owed (left; copyEachStart on the
// first pass). False: a copy's election is posed, and copyResume calls back
// with the count still owed.
//
//go:noinline
func (s *State) copyEachOther(k *prep, in *kop.Instr, left int) bool {
	src := s.objRefObj(in.Obj)
	si := s.spellStackIndex(src)
	if si < 0 {
		return true
	}
	ctrl := s.res.e.ctrl
	ex := s.playerRef(k, in.W)
	var who seatSeq
	if ex == 255 {
		who.add(ctrl) // no caster read: gorge keeps the resolving controller
	} else {
		all := s.aliveFrom(ctrl)
		for i := uint8(0); i < all.n; i++ {
			if p := all.at(i); p != ex {
				who.add(p)
			}
		}
	}
	if left == copyEachStart {
		left = int(who.n)
	}
	for left > 0 && left <= int(who.n) {
		if !s.mintSpellCopy(k, si, who.at(who.n-uint8(left))) {
			return true
		}
		left--
		if in.C&1 != 0 && s.copyElect(k, uint8(left)) { // seat-ok: an instruction flag bit
			return false
		}
	}
	return true
}
