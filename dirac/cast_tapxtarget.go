package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// tapXTargetAbort reports whether the pending activation can no longer be
// completed because its committed tap-another picks (a tapXType cost part)
// took the mana sources the rest of the cost needs: the cost is unpayable from
// the pool and the CR 601.2g window with the picked permanents withheld.
// Gorge's affordableTargetCandidates probes the same window after the picks
// are announced (the picks are no window unit, as convokeCommitted), so a
// verdict of probeManaShort withholds every candidate: a required target
// reverses the activation ("cast aborted: no legal target", CR 733.1) and an
// up-to target asks nothing. An incomplete census keeps the offer. The shape
// is waterbendTargetAbort's, for the tap-another pick of an ability.
func (s *State) tapXTargetAbort(k *prep) bool {
	e := &s.bld().e
	if k.gen == nil || e.kind != seAbility || e.ab == 255 || !s.committedTapXPicks().any() {
		return false
	}
	if _, ok := s.waterbendPartFor(k, e); ok {
		return false // waterbendTargetAbort's
	}
	cost := xFolded(s.buildCost(k), int(e.x))
	if cost == (kop.ManaCost{}) {
		return false
	}
	saved := s.fork()
	defer s.unfork()
	s.occ = s.occ.andNot(s.committedTapXPicks())
	if !s.windowCensusComplete(k, e) {
		s.CopyFrom(saved)
		return false
	}
	ok, _ := s.windowTapRider(k, e.ctrl, &cost, false, s.bld().bfl&bflAnyColor != 0)
	s.CopyFrom(saved)
	return !ok
}

// committedTapXPicks is the battlefield slots of the build's tap-another picks.
func (s *State) committedTapXPicks() slotMask {
	var m slotMask
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkTapX && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield {
			m = m.with(s.objs[s.bld().picks[i]].slot)
		}
	}
	return m
}
