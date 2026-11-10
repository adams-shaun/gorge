package dirac

// ovSelfGrants reports whether a live override record on sl, made by sl's
// own card c and not older than sl's loss of all abilities, grants c's
// ability ab: the permanent is then its own granting host (The Irencrag's
// Animate gives Everflame equip {3}; gorge registers the granted keyword
// ability on the animated object itself). activateGen reads the host's
// card, so only c == the permanent's card qualifies.
func (s *State) ovSelfGrants(k *prep, sl Slot, c CardID, ab uint8) bool {
	if !s.fxM.has(sl) || s.cardAt(sl) != c {
		return false
	}
	lts, lost := s.lossAt(k, sl)
	hit := false
	s.eachOvRec(sl, func(r ovRec) {
		if r.src != c || lost && r.ts < lts {
			return
		}
		if g := k.t.Overrides[r.idx-1].Grant; g[0] == ab+1 || g[1] == ab+1 {
			hit = true
		}
	})
	return hit
}
