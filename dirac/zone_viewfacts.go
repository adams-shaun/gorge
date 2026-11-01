package dirac

// FeatLandDropOpen reports whether p may still play a land this turn, the extra
// land drops of its permanents counted.
func (s *State) FeatLandDropOpen(p uint8) bool {
	return s.pl[p].landsPlayed < s.landLimit(s.k(), p)
}
