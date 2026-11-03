package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A mana ability that adds several colours at once (kop.Ability.ManaEach:
// Timeless Lotus's "{T}: Add {W}{U}{B}{R}{G}", CR 106.1). gorge's planner
// prices it as one fixed-production alternative whose Alt.Mana holds every
// colour (pay appendUnitAlternatives), so its rank keys read all of them:
// flexibility the distinct types it makes, the remainder's colour sources
// each colour, the surplus the whole production. That is exactly the shape
// of a tap's extra mana (mana_tapextra.go): a unit of one colour beside a
// fixed vector the tap also adds. The kernel splits the production so: the
// lowest colour is the unit's (pcard.manaCol, a one-colour mask, so the
// source is a deterministic unit in gorge's census order), the rest is
// pcard.manaXv, which srcExtra and srcExtraPick add to any carrier's extra.
// kgen admits only the plain shape (refuseEachMana): the card's one mana
// ability, a bare {T}, unrestricted, a literal amount of each colour.

// prepEachMana splits all-at-once ability a's production into pc's unit
// colour and its fixed rest.
func prepEachMana(a *kop.Ability, pc *pcard) {
	m := uint8(a.ManaColors) & 0x3f
	first := bits.TrailingZeros8(m)
	pc.manaCol = kop.ColorMask(1) << first
	for c := first + 1; c < kop.NumManaKinds; c++ {
		if m&(1<<c) != 0 {
			pc.manaXv[c] = pc.manaN
		}
	}
}

// addOwnXv adds pc's all-at-once rest to the extra xv.
func addOwnXv(xv *[kop.NumManaKinds]uint8, pc *pcard) {
	for c := range xv {
		xv[c] += pc.manaXv[c]
	}
}

// xvMask is the colours of pc's all-at-once rest (each pc.manaN of them).
func xvMask(pc *pcard) uint8 {
	var m uint8
	for c, n := range pc.manaXv {
		if n > 0 {
			m |= 1 << c
		}
	}
	return m
}

// windowEachMask is the colours an all-at-once source at sl adds beside its
// unit's colour: the window's usefulness clamp reads its whole production,
// while the colour it is tapped "for" stays the unit's (it asks no colour).
func (s *State) windowEachMask(k *prep, sl Slot) uint8 {
	if !k.manaEach {
		return 0
	}
	return xvMask(s.manaPCAt(k, sl))
}
