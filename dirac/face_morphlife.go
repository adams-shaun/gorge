package dirac

// A face-down keyword whose turn-up cost is a life payment
// (K:Morph:PayLife<N>, Zombie Cutthroat; CR 702.37e, 119.4). gorge's keyword
// turn-up (rules morph_turnup.go) prices the cost's Life part with its mana:
// pay.CostPayableSpecial withholds the action while the controller's life is
// below N, and pay.PayManaSpecial charges the life with the mana before the
// TurnFaceUp event. CardDef.MorphLife carries N (MorphCost is {0}); only a
// keyword procedure pays it, never a manifested or cloaked card's printed
// mana cost (CR 701.40b, 701.58b).

// turnUpLifeOK reports whether p's life covers the keyword turn-up life of
// the face-down permanent at sl (true when it has none).
func (s *State) turnUpLifeOK(k *prep, p uint8, sl Slot) bool {
	n := k.t.Cards[s.cardAt(sl)].MorphLife
	return n == 0 || int(s.pl[p].life) >= int(n)
}

// payTurnUpLife pays the keyword turn-up life of the permanent at sl: p
// loses it (CR 119.4), after the mana and before the permanent turns up.
func (s *State) payTurnUpLife(k *prep, p uint8, sl Slot) {
	s.loseLife(k, p, int(k.t.Cards[s.cardAt(sl)].MorphLife))
}
