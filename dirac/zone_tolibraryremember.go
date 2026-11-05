package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// toLibraryRemember is OpToLibrary with kop.LibBottomRememberGrave: the trigger
// object, if it is still in a graveyard, goes to the bottom of its owner's
// library, and a card that reached the library joins the remembered set (gorge
// effChangeZone: the Origin$ Graveyard precondition, then RememberChanged$ for
// each object that moved).
//
//go:noinline
func (s *State) toLibraryRemember(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneGraveyard {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, true)
	if s.objs[o].zone == kop.ZoneLibrary {
		s.remember(o)
	}
}
