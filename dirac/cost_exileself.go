package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// exileSelfCost pays a CostExileSelf (gorge Exile<1/CARDNAME>: the source
// moves to exile as the cost is paid; its leaves-the-battlefield triggers
// fire, it is not sacrificed; its last known information is kept as a
// sacrifice-this's is). A source that could tap for mana is held
// back until after the mana, as a sacrifice-this (returnSelfDeferred).
//
//go:noinline
func (s *State) exileSelfCost(k *prep, sl Slot) {
	if k.gen != nil {
		// last known information, as a sacrifice-this: Self's counters
		// and P/T read the exiled source as it last existed (CR 608.2h)
		s.bld().e.li = s.snapLKI(s.perms[sl].obj)
		if s.deferSacGen(k, sl) {
			return
		}
	}
	s.leave(k, sl, kop.ZoneExile, false)
}
