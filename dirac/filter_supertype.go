package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// superGen reads the supertype filter bits (F2Basic, F2Legendary) against
// the permanent's current types (gorge hasType).
func (s *State) superGen(k *prep, f kop.FilterFlags2, sl Slot) bool {
	t := s.typesGen(k, sl)
	return (f&kop.F2Basic == 0 || t&kop.TBasic != 0) && (f&kop.F2Legendary == 0 || t&kop.TLegendary != 0)
}
