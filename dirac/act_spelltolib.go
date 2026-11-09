package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// spellToLib is ExtSpellToLib: the resolving spell goes from the stack to the
// top of its owner's library and that library is shuffled (CR 608.2n; gorge
// effChangeZone over Defined$ Parent, Shuffle$ True). The object is off the
// stack, so finishResolve moves it nowhere else. A spell copy stays on the
// stack for finishResolve to cease (spellRestExiles), and so does a flashback
// spell, which is exiled whenever it would leave the stack (CR 702.34a, gorge
// keywordExileDeparture); the owner's library is shuffled either way (CR
// 701.24c, objectPathShuffleOwners).
//
//go:noinline
func (s *State) spellToLib(k *prep) {
	e := &s.res.e
	if e.kind != seSpell {
		return
	}
	o := e.obj
	if ob := &s.objs[o]; ob.zone == kop.ZoneStack && ob.of&ofCopy == 0 && e.flags&sfFlashback == 0 {
		s.touch(o)
		ob.zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneLibrary, false, kop.ZoneStack, false)
	}
	s.shuffleLib(s.objs[o].owner())
}
