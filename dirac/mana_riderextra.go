package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Extra mana a bounded rider source's tap brings. A mana ability whose cost
// taps and carries a rider (Remote Farm's "{T}, remove a depletion counter:
// add {W}{W}", sacrificed on its last counter) is a tap for mana like any
// other (CR 605.1a), so a StatTapExtra static reaching the permanent (Wild
// Growth on it: "its controller adds an additional {G}") adds its mana beside
// the ability's own. gorge folds the extra into the relaxed alternative
// (potential_plan.go potentialTapExtra), so the census unit
// (addTapsDeferUnits) counts it and the activation (manaRiderFloat) floats
// it. Only a literal carrier (a fixed symbol) is priced here; a reflected or
// {C}-limited one depends on the colour made and stalls (StallTapExtra).

// riderTapExtra is the literal extra the StatTapExtra statics add when the
// rider source at sl taps for mana. False when a carrier that depends on the
// colour made reaches it.
func (s *State) riderTapExtra(k *prep, sl Slot) (xv [kop.NumManaKinds]uint8, ok bool) {
	ok = true
	ctl := s.perms[sl].ctrl
	s.eachStatic(k, func(st statSrc) bool {
		a := &st.a.Static
		if !s.tapExtraReaches(k, st, sl, ctl) {
			return true
		}
		if a.StFlags&(kop.StTapReflect|kop.StTapOnC) != 0 {
			ok = false
			return false
		}
		xv[a.A] += uint8(a.B)
		return true
	})
	if !ok {
		return [kop.NumManaKinds]uint8{}, false
	}
	return xv, true
}

// riderExtraUnits adds the extra of the rider source at sl's tap to av.
func (s *State) riderExtraUnits(k *prep, sl Slot, av *avail) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv, ok := s.riderTapExtra(k, sl)
	if !ok {
		s.capFull(StallTapExtra)
		return
	}
	for c, v := range xv {
		if v > 0 {
			av.addUnits(1<<c, v)
		}
	}
}

// riderExtraFloat floats the extra of the rider source at sl's tap in p's pool.
func (s *State) riderExtraFloat(k *prep, p uint8, sl Slot) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return
	}
	xv, ok := s.riderTapExtra(k, sl)
	if !ok {
		s.capFull(StallTapExtra)
		return
	}
	for c, v := range xv {
		s.pl[SeatIx(p)].pool[c] += v
	}
}
