package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// excludePickedSources takes out of the X ask's census (availXWindow) each
// permanent this build already committed to a waterbend or convoke pick: the
// pick taps it to pay a generic of the cost, so it cannot also tap for mana
// (CR 701.67a, 702.51a, 601.2g-h). The payment window skips it
// (windowPick), and gorge's castWindowUnits drops it (convokeCommitted).
func (s *State) excludePickedSources(k *prep, av *avail) {
	b := s.bld()
	for i := 0; i < int(b.pickN); i++ {
		if b.pickKs[i] != pkWaterbend && !isConvokePick(b.pickKs[i]) {
			continue
		}
		ob := &s.objs[b.picks[i]]
		if ob.zone != kop.ZoneBattlefield {
			continue
		}
		sl := ob.slot
		if !s.canTapForMana(k, sl) {
			continue
		}
		pc := &k.cards[s.cardAt(sl)]
		if k.gen != nil {
			pc = s.manaPCGen(k, sl)
		}
		s.removeSrc(k, av, sl, pc)
	}
}

// pickedForMana reports whether o is a Convoke or Waterbend contribution of
// the build in progress: it is tapped to pay that cast (CR 702.51a), so it
// cannot also tap for mana and the cast's mana census must leave it out.
func (s *State) pickedForMana(o Obj) bool {
	if !s.buildLive() {
		return false
	}
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().picks[i] == o && (s.bld().pickKs[i] == pkWaterbend || isConvokePick(s.bld().pickKs[i])) {
			return true
		}
	}
	return false
}
