package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// toLibraryRemember is OpToLibrary with kop.LibBottomRememberGrave or
// kop.LibTopRememberGrave: the trigger object or target, if it is still in a
// graveyard, goes to the bottom (bottom true) or the top of its owner's
// library, and a card that reached the library joins the remembered set (gorge
// effChangeZone: the Origin$ Graveyard precondition, then RememberChanged$ for
// each object that moved).
//
//go:noinline
func (s *State) toLibraryRemember(k *prep, in *kop.Instr, bottom bool) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneGraveyard {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, bottom)
	if s.objs[o].zone == kop.ZoneLibrary {
		s.remember(o)
	}
}
