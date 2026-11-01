package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Omen cards (CR 720), cold: reached only when the table
// holds a FaceOmen card (genPrep.anyFaces) and one is cast as gorge's
// "omen_alt" (dirac/face_castface.go offerFaceCasts). A resolved Omen
// goes to the BOTTOM of its owner's library (gorge's MoveZone appends to the
// destination zone, and the library top is index 0), then that library is
// shuffled (gorge rules resolution_chain.go moveResolvedOffStack). Countered
// or fizzled, it goes to its owner's graveyard as its front, with no shuffle
// (gorge rules stack.go spellFizzleZone).

// DebugOmenGraveyard makes a resolved Omen rest in the graveyard instead of
// the library (mutation testing only: the lockstep must catch it).
var DebugOmenGraveyard bool

// DebugOmenNoShuffle skips the shuffle after a resolved Omen reaches the
// library (mutation testing only).
var DebugOmenNoShuffle bool

// DebugOmenTop puts a resolved Omen on top of the library instead of the
// bottom (mutation testing only).
var DebugOmenTop bool

// omenRest returns the zone a leaving Omen spell rests in: the library when
// it resolved, its own zone otherwise (a counter, a fizzle, a redirected
// counter and a reversal keep their destination). The exile arm of gorge's
// spellRestZone wins before FlagOmen; an Omen cast from the hand carries
// none of those flags, so z is the graveyard here.
func (s *State) omenRest(ob *object, z kop.Zone, resolved bool) kop.Zone {
	if !DebugOmenGraveyard && resolved && z == kop.ZoneGraveyard && ob.of&(ofCopy|ofToken) == 0 {
		return kop.ZoneLibrary
	}
	return z
}

// omenToLibrary moves a resolved Omen off the stack to the bottom of its
// owner's library and shuffles it (gorge's moveResolvedOffStack: the move
// first, then the owner's Shuffle). DebugOmenTop puts it on top instead and
// DebugOmenNoShuffle skips the shuffle.
func (s *State) omenToLibrary(o Obj) {
	s.placeFrom(o, kop.ZoneLibrary, !DebugOmenTop, kop.ZoneStack, false)
	if !DebugOmenNoShuffle {
		s.shuffleLib(s.objs[o].owner())
	}
}
