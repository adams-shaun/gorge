package dirac

// shuffleRememberedOwners is OpShuffle with C != 0 (gorge effShuffle over
// definedSpecRememberedOwner, ownersOf): each distinct owner of a remembered
// object shuffles their library, in remembered order. Nobody shuffles with
// nothing remembered. An object's owner never changes (CR 108.3).
//
//go:noinline
func (s *State) shuffleRememberedOwners() {
	var seen uint32
	for i := 0; i < int(s.g.nrem); i++ {
		p := s.objs[s.g.rem[i]].owner()
		if int(p) >= MaxSeats || seen&(1<<p) != 0 {
			continue
		}
		seen |= 1 << p
		s.shuffleLib(p)
	}
}
