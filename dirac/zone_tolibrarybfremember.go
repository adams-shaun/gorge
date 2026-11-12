package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// toLibraryRememberBF is OpToLibrary with kop.LibBottomRememberBF (bottom) or
// kop.LibTopRememberBF (the top): the object
// goes to its owner's library only if it is still on the
// battlefield (gorge effChangeZone: the Origin$ Battlefield precondition), and
// a card that reached the library joins the remembered set (RememberChanged$).
//
//go:noinline
func (s *State) toLibraryRememberBF(k *prep, in *kop.Instr, bottom bool) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, bottom)
	if s.objs[o].zone == kop.ZoneLibrary {
		s.remember(o)
	}
}
