package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// toLibraryShuffle is OpToLibrary with kop.LibTopShuffle (gorge
// effChangeZone's object path with Shuffle$ True): the object goes on top
// of its owner's library (Forge's absent LibraryPosition$), then that
// library is shuffled -- even when the object did not move (CR 701.24c,
// objectPathShuffleOwners), as long as the object still exists. With bottom
// (kop.LibBottomShuffle, LibraryPosition$ -1) the object goes on the bottom.
//
// Defined$ Enchanted/Equipped is RefAttached. When the source is off the
// battlefield, gorge's definedSpecEquipped (engine/effects/context.go:
// 1306-1328) falls back to Object.LastBearer (CR 608.2h): the referent can
// be a creature that already left, and objectPathShuffleOwners
// (engine/effects/zone_library.go:83-97) still shuffles its owner's
// library. Only a referent still on the battlefield is moved (Origin$
// Battlefield).
//
//go:noinline
func (s *State) toLibraryShuffle(k *prep, in *kop.Instr, bottom bool) {
	o := s.objRefObj(in.Obj)
	lastBear := false
	if o == NoObj && in.Obj == kop.RefAttached && s.objs[s.res.e.obj].zone != kop.ZoneBattlefield {
		o = s.lastBearerObj(s.res.e.obj)
		lastBear = o != NoObj
	}
	if o == NoObj || in.C == kop.LibTopShuffleGrave && s.objs[o].zone != kop.ZoneGraveyard {
		return // (Origin$ Graveyard: a card exiled in response is not shuffled in, nor the library)
	}
	if bottom && s.objs[o].zone != kop.ZoneBattlefield {
		// Origin$ Battlefield: an object that already left (a second trigger
		// of the source that went to the library first) does not move, and
		// the library is shuffled all the same (CR 701.24c)
	} else if !lastBear {
		s.moveRef(k, in.Obj, kop.ZoneLibrary, bottom)
	} else if s.objs[o].zone == kop.ZoneBattlefield {
		// the last bearer returned: gorge's LastBearer names the returned
		// object, so the Origin$ Battlefield move still applies.
		s.leave(k, s.objs[o].slot, kop.ZoneLibrary, bottom)
	}
	if s.res.ask == raGenCmdZone {
		return // a CR 903.9b commander move parked: the shuffle waits for the answer
	}
	if in.Obj == kop.RefAttached && DebugEquipShufNoShuffle {
		return
	}
	s.shuffleLib(s.objs[o].owner())
	if DebugEquipShuf.On {
		s.noteEquipShuf(o, lastBear)
	}
	if DebugLibposLog {
		LibposCount[LibposShuffle]++ // zone_librarysecond.go
	}
}

// libTopInOrder puts the detached cards os on top of their owners'
// libraries so that os[0] ends on top: gorge's ChangeZoneAll top placement
// (libraryOrderPlacement: the moved cards in move order, then the rest).
// A sequential place onto the top would leave the last one on top. from is
// each card's pre-move zone, recorded with its entry.
//
//go:noinline
func (s *State) libTopInOrder(os []Obj, from []kop.Zone) {
	for i := len(os) - 1; i >= 0; i-- {
		s.placeFrom(os[i], kop.ZoneLibrary, false, from[i], false)
	}
}

// reverseLook reverses the picks of a hand move to the library before
// moveFound places them one by one on top, so the first answered card ends
// on top (gorge handLibraryTail -> libraryOrderPlacement: the moved cards
// on top in answer order; Brainstorm).
//
//go:noinline
func (s *State) reverseLook() {
	l := s.resArm().look[:s.g.nlook]
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
}

// toLibraryPair is OpToLibraryPair (Floodpits Drowner's "shuffle this and
// target creature into their owners' libraries"): the source reaches its library
// ahead of the target, then each distinct owner's library is shuffled once, the
// target's owner first (gorge objectPathShuffleOwners). The move order is the
// shuffle's input.
func (s *State) toLibraryPair(k *prep, in *kop.Instr) {
	first, src := s.objRefObj(in.Obj), s.objRefObj(kop.RefSelf)
	s.moveRef(k, kop.RefSelf, kop.ZoneLibrary, false)
	if s.res.ask == raGenCmdZone {
		return // a CR 903.9b commander move parked: the target and the shuffle wait
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, false)
	if s.res.ask == raGenCmdZone {
		return
	}
	p1 := uint8(255)
	if first != NoObj {
		p1 = s.objs[first].owner()
		s.shuffleLib(p1)
	}
	if src != NoObj && s.objs[src].owner() != p1 {
		s.shuffleLib(s.objs[src].owner())
	}
}
