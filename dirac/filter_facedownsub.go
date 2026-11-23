package dirac

// faceDownHasSub is permHasSub's answer for a face-down permanent read above
// layer 3. A face-down permanent has no subtypes of its own (CR 708.2), but an
// effect that gives it creature types still does (CR 613.1d: layer 4 applies
// over the face-down values): Mirror Entity's "creatures you control gain all
// creature types" makes a manifested card a Zombie, and gorge's layer walk
// reads it so. The walk starts a face-down slot from no subtypes
// (charsFx), so with no effect on the slot the answer stays false.
func (s *State) faceDownHasSub(k *prep, sl Slot, i uint8) bool {
	if k.gen == nil || !s.fxM.has(sl) && k.gen.fxNeed == 0 {
		return false
	}
	return s.walkHasSub(k, sl, i)
}
