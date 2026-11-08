package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// toLibraryGrave is OpToLibrary with kop.LibTopGrave or kop.LibBottomGrave:
// the trigger object goes on the top or the bottom of its owner's library
// only if it is still in a graveyard (gorge effChangeZone's Origin$
// Graveyard precondition). A card returned to a hand in response is a new
// object the ability cannot find (CR 400.7); Fell Horseman's dies trigger
// after Edgewall Inn returned it.
//
//go:noinline
func (s *State) toLibraryGrave(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.objs[o].zone != kop.ZoneGraveyard {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, in.C == kop.LibBottomGrave)
}
