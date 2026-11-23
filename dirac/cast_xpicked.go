package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// pickCommittedSrc reports whether the build has committed the permanent at
// sl to a waterbend or convoke pick: the pick taps it to pay a generic of the
// cost, so it cannot also tap for mana (CR 701.67a, 702.51a, 601.2g-h). The
// window census's gap arms (availXWindow's deferred, Combo Any, life-gap and
// Taps-deferred sources, which availFor leaves out) skip it — removeSrc
// cannot take their units out, the planner prices none for a gap source — and
// gorge's castWindowUnits drops the committed source outright
// (convokeCommitted).
//
// Only a live build holds picks: the scratch keeps an aborted cast's picks
// (CR 733.1) after reverseCast, and a priority decision's census must not read
// them as committed.
func (s *State) pickCommittedSrc(k *prep, sl Slot) bool {
	if !s.buildLive() {
		return false
	}
	b := s.bld()
	for i := 0; i < int(b.pickN); i++ {
		if b.pickKs[i] != pkWaterbend && !isConvokePick(b.pickKs[i]) {
			continue
		}
		if ob := &s.objs[b.picks[i]]; ob.zone == kop.ZoneBattlefield && ob.slot == sl {
			return true
		}
	}
	return false
}

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
		// A Combo Any or reflected Amount$ source is a census gap: the plain
		// walk leaves it out (manaAb < 0) but availXWindow still counts its
		// units through availCA, so canTapForMana is false for it. The pick
		// taps it for the payment all the same, and the X window cannot spend
		// that tap again (CR 601.2g-h): take its window units out here too.
		s.removeCA(k, av, sl)
		// A sacrifice-only source (an Eldrazi Scion) is a census unit with no
		// {T}: the pick taps its creature, so the sacrifice is gone too
		// (gorge's convokeCommitted drops it from the window's units).
		if !s.canTapForMana(k, sl) && !s.canSacOnlyAt(k, sl) {
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
//
// It inlines to the two guards (ctl is a build or a build's ask, a superset of buildLive) and one call.
func (s *State) pickedForMana(o Obj) bool {
	return (s.ctl == ctlBuild || s.ctl == ctlGenAsk) && s.pickedForManaScan(o)
}

//go:noinline
func (s *State) pickedForManaScan(o Obj) bool {
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
