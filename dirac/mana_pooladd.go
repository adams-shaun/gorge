package dirac

// poolAdd adds n mana of kind col to player p's pool, saturating at 255 (a
// pool colour is a byte in State) and stalling StallDynMana when the sum
// would pass it: a wrapped pool would silently lose mana. n <= 0 adds
// nothing. One player's pool holds at most 255 of one mana kind; more stalls
// loudly (the declared ceiling, see maxDynMana).
func (s *State) poolAdd(p uint8, col, n int) {
	if n <= 0 {
		return
	}
	t := int(s.pl[p&1].pool[col]) + n
	if t > 255 {
		t = 255
		s.capFull(StallDynMana)
	}
	s.pl[p&1].pool[col] = uint8(t)
}
