package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugPutLibShuffle is TestDanielsMutation's switch: the put-on-bottom cost
// also shuffles its owner's library, which gorge does not do for a cost.
var DebugPutLibShuffle bool

// payPutLibPicks pays the chosen PutCardToLibFromGrave<1/-1/CARDNAME> cost
// (kop.Cost.PutLibSelf; pkPutLib): the source card in its owner's graveyard
// goes to the bottom of that library as the ability's cost (CR 118.3,
// 602.2b; Daniels, Temporal Agent). It is no effect, so no replacement
// shuffles the library in; the other cards keep their order.
func (s *State) payPutLibPicks(k *prep) {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkPutLib {
			continue
		}
		o := s.bld().picks[i]
		ob := &s.objs[o]
		if ob.zone != kop.ZoneGraveyard {
			continue
		}
		owner := ob.owner()
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneLibrary, true, from, false)
		if DebugPutLibShuffle {
			s.shuffleLib(owner)
		}
	}
}
