package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterTo is counterAt with Counter's Destination$ (gorge effCounter): a
// countered spell card goes to its owner's hand, exile or library top or
// bottom (kop.CtrDest*) instead of its graveyard; a flashback spell is
// exiled whatever the destination (CR 702.34a, gorge
// state.ExilesLeavingStack). An ability is removed as counterAt does.
//
//go:noinline
func (s *State) counterTo(k *prep, i int, dest int16) {
	e := s.stack[i]
	if e.kind != seSpell || e.flags&sfFlashback != 0 {
		s.counterAt(k, i)
		return
	}
	copy(s.stack[i:s.sp-1], s.stack[i+1:s.sp])
	s.sp--
	s.touch(e.obj)
	s.objs[e.obj].zone = kop.ZoneNone
	z, bottom := kop.ZoneLibrary, true
	switch dest {
	case kop.CtrDestExile:
		z, bottom = kop.ZoneExile, false
	case kop.CtrDestHand:
		z, bottom = kop.ZoneHand, false
	case kop.CtrDestLibTop:
		z, bottom = kop.ZoneLibrary, false
	}
	if k.gen != nil && k.gen.anyFaces {
		// every exit from the stack is its front again; a countered
		// Adventure is not on an adventure (face_castface.go)
		z = s.stackExitFace(k, e.obj, z, false)
	}
	s.placeFrom(e.obj, z, bottom, kop.ZoneStack, false)
}
