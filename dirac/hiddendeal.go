package dirac

// DealHiddenOrdered deals pool (len = p's hand + library size) to player
// p's hidden objects in the order given: pool[:hand size] to the hand,
// the rest to the library top first. a search's learned beliefs
// (:deckbelief, :handbelief) choose the hand themselves, so unlike
// DealHidden nothing is shuffled here. It is out of line.
func (s *State) DealHiddenOrdered(p uint8, pool []CardID) {
	pl := &s.pl[p]
	h := int(pl.handN)
	for i := 0; i < h && i < len(pool); i++ {
		s.dealCard(pl.hand[i], pool[i])
	}
	if h > len(pool) {
		h = len(pool)
	}
	rest := pool[h:]
	for i := 0; i < len(rest) && int(pl.libLo)+i < int(pl.libHi); i++ {
		s.dealCard(pl.lib[int(pl.libLo)+i], rest[i])
	}
}
