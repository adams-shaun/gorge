package dirac

// randomDiscardPick records one Discard<N/Random> cost pick of the entry being
// built (kop.Cost.DiscardRandom): the engine RNG draws an index over the
// still-eligible hand cards in hand order (d holds them, costCands), one draw
// per card, as gorge discardAsk's Random arm (e.Rand over the shrinking
// candidate list with order-preserving removal; act_discardrandom.go's pick).
// Nothing is asked: the card is a recorded pick the payment discards in pick
// order. d must hold at least one candidate (the caller reverses a short cost).
func (s *State) randomDiscardPick(d *Decision) {
	j := s.rng.IntN(int(d.N))
	s.recordPick(Obj(d.Options[j].Source()))
}
