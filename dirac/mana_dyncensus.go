package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// dynZeroCensusAb: a is a plain {T} mana ability with a dynamic amount
// (Alena, Kessig Trapper's red equal to the greatest power among creatures
// you control that entered this turn): gorge's window census prices it by
// resolving its Amount$ now (castWindowAmount), and an amount of zero or less
// is no unit.
func dynZeroCensusAb(a *kop.Ability) bool {
	return a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.AFlags&kop.AfManaDynamic != 0 &&
		a.ManaRestrict == 0 && a.ManaRestrictAb == 0 && a.Cost.Flags == kop.CostTap &&
		a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// prepDynZero reports whether some card of t has a dynZeroCensusAb ability.
func prepDynZero(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if dynZeroCensusAb(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// dynZeroCensusGap reports whether p controls an untapped source whose only
// mana is a dynamic amount that resolves to nothing now. gorge's census leaves
// it unpriced and counts that as a gap (castWindowSourceExcluded's
// unresolvable Amount$ arm), so the census is incomplete and a pool-short
// target candidate stays on the menu (affordableTargetCandidates, CR 601.2g).
// A source with another mana ability that yields is priced by that one.
func (s *State) dynZeroCensusGap(k *prep, p uint8) bool {
	for m := s.ctrlM[p].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) ||
			k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		zero, yields := false, false
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			if dynZeroCensusAb(a) && s.evalV(k, a.ManaV, evalCtx{src: s.perms[sl].obj, srcSl: sl, ctrl: s.perms[sl].ctrl}) <= 0 {
				zero = true
				continue
			}
			yields = true
		}
		if zero && !yields {
			return true
		}
	}
	return false
}
