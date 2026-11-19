package dirac

// A gate-on pair source that also holds a granted mana ability (Whisperer of
// the Wilds under Cryptolith Rite: {T}: Add {G}, the ferocious {T}: Add {G}{G},
// the grant's {T}: Add one mana of any colour). gorge's prefix search asks
// "choose a mana ability" over all three, the larger pair alternative second
// (CR 605.1a), and a line that taps it for {G}{G} and then every other grant
// holder for its colour pays a play the printed sources alone cannot. The
// grant search branches the pair's larger alternative beside the printed one
// (gatedPairWide), in the printed pair's place in that ask.

// gatedPairWide is the mana the gate-on pair view of sl makes with its larger
// {T} ability, 0 when sl's view is not a pair (widePairAb).
func (s *State) gatedPairWide(k *prep, sl Slot) uint8 {
	pc := s.manaPCGen(k, sl)
	if s.widePairAb(k, sl, pc) < 0 {
		return 0
	}
	return pc.wideN
}

// gatedPairMask is the colour of sl's larger pair alternative.
func (s *State) gatedPairMask(k *prep, sl Slot) uint8 {
	return uint8(s.manaPCGen(k, sl).wideCol) & 0x3f
}
