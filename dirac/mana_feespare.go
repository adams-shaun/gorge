package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// feeSpareCovers is availPotentialFor's read of gorge's potentialFeeSpareCovers
// (CR 106.6, 605.3a): the paid mana ability j of the source at sl, whose fee the
// walk's other mana (others) leaves short for payment d, is still one gorge's
// bound admits when p's restricted sources that d refuses but an activation of
// sl admits (a Powerstone's {C} refused for a nonartifact spell) pay the fee.
// That mana cannot pay the spell yet pays Urn of Godfire's "{2}: Add one mana
// of any color", whose own mana then pays it.
func (s *State) feeSpareCovers(k *prep, p uint8, sl Slot, ab *kop.Ability, d *payDesc, others *[kop.NumManaKinds]int) bool {
	if d == nil || !k.restrSrc {
		return false
	}
	act := payDesc{class: pdAct, card: s.cardAt(sl), src: sl}
	with, spare := *others, false
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		o := m.first()
		if o == sl {
			continue // a source never funds its own fee
		}
		cd := &k.t.Cards[s.cardAt(o)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if j >= 16 || a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 || a.ManaRestrict == 0 ||
				s.potentialAdmits(k, p, o, j, a, d) || !s.potentialAdmits(k, p, o, j, a, &act) {
				continue
			}
			for mk := uint8(a.ManaColors); mk != 0; mk &= mk - 1 {
				with[bits.TrailingZeros8(mk)] += int(max(a.ManaN, 1))
				spare = true
			}
		}
	}
	return spare && potentialCovers(&with, &ab.Cost.Mana)
}
