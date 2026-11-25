package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An all-at-once restricted source (Troyan, Gutsy Explorer's "{T}: Add {G}{U}.
// Spend this mana only to cast ...") is one planner unit of its lowest colour
// whose other colours ride the unit's xv (the tap's fixed rest). Every mana of
// the ability carries the restriction (CR 106.6), so a plan tapping it floats
// the whole production restricted, as gorge's one activation does: the rest
// left floating after the payment is as restricted as the colour spent, and no
// cast its restriction does not match may spend it.

// floatRestrEach floats every colour of all-at-once restricted ability ab (n
// of each) restricted beside the pool and returns the colours other than col,
// the part of the unit's xv the caller must not also add to the plain pool.
func (s *State) floatRestrEach(k *prep, p uint8, sl Slot, ab *kop.Ability, col int, n uint8) (rest [kop.NumManaKinds]uint8) {
	if !s.floatRestrAb(k, p, sl, ab, col, n) { // ManaEach floats every colour: col is unread
		s.capFull(StallRestrSrc)
	}
	for c := 0; c < 5; c++ {
		if c != col && ab.ManaColors&(1<<c) != 0 {
			rest[c] = n
		}
	}
	return rest
}

// addXvPickLess is addXvPick over ty.xv less own, the mana floatRestrEach
// floated restricted already.
func (s *State) addXvPickLess(p uint8, ty *utype, col int, own *[kop.NumManaKinds]uint8) bool {
	if *own == [kop.NumManaKinds]uint8{} {
		return s.addXvPick(p, ty, col)
	}
	for c := range ty.xv {
		s.pl[p].pool[c] += ty.xv[c] - min(ty.xv[c], own[c])
	}
	return ty.pick && s.reflectPickPark(p, ty, col)
}
