package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Banding's grant read (CR 702.22). KwMask is full (kop/keywords.go), so a
// granted banding is kop.Override.Banding on the active override records,
// not a keyword bit: kwGen's mask cannot carry it and the projection maps
// the keyword beside the record, like CantAttack. combat_band.go's CR 702.22c
// band ask and band formation (CR 702.22d-k) call this read. The
// record's lifetime is the grant's: a Pump Duration$
// UntilEndOfCombat record carries UntilEndCombat and static_ovendcombat.go
// drops it leaving the end-of-combat step.

// hasBanding reports whether an active override record grants sl banding. A
// grant older than the permanent's "loses all abilities" effect is removed
// with the rest of layer 6 (kwGen's override loop).
func (s *State) hasBanding(k *prep, sl Slot) bool {
	lts, lost := s.lossAt(k, sl)
	found := false
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if (!lost || ts >= lts) && o.Banding {
			found = true
		}
	})
	return found
}
