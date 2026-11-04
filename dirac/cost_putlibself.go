package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugPutLibShuffle is TestDanielsMutation's switch: the put-on-bottom cost
// also shuffles its owner's library, which gorge does not do for a cost.
var DebugPutLibShuffle bool

// putLibSelfCost pays PutCardToLibFromGrave<1/-1/CARDNAME> (kop.Cost.PutLibSelf):
// the source card in its owner's graveyard goes to the bottom of that
// library as the ability's cost (CR 118.3, 602.2b; Daniels, Temporal Agent).
// It is no effect, so no replacement shuffles the library in; the other
// cards keep their order and the source is a new object in the library.
func (s *State) putLibSelfCost(k *prep, e *stackEntry) {
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneGraveyard {
		return
	}
	owner := ob.owner()
	from := s.detach(e.obj)
	s.placeFrom(e.obj, kop.ZoneLibrary, true, from, false)
	if DebugPutLibShuffle {
		s.shuffleLib(owner)
	}
}
