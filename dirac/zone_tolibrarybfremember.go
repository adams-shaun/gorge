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
	s.toLibraryRememberFrom(k, in, bottom, kop.ZoneBattlefield)
}

// toLibraryRememberFrom is the remembering library move from origin: the
// object moves only if it is still in origin (gorge effChangeZone's Origin$
// precondition), and a card that reached the library joins the remembered
// set (RememberChanged$). kop.LibBottomRememberExile reads it from exile
// (Time Reaper).
//
//go:noinline
func (s *State) toLibraryRememberFrom(k *prep, in *kop.Instr, bottom bool, origin kop.Zone) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != origin {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, bottom)
	if s.objs[o].zone == kop.ZoneLibrary {
		s.remember(o)
	}
}
