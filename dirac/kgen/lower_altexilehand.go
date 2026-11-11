package kgen

// altPayLifeExileHand: the lowered cost of a spell's own AlternativeCost is
// PayLife<N> and ExileFromHand<N/Spec> and nothing else (Force of Will: "You
// may pay 1 life and exile a blue card from your hand rather than pay this
// spell's mana cost", CR 118.9, 601.2b). The two parts share kop.Cost.N, so
// the life equals the exiled count; build_ab's AlternativeCost arm gates the
// card (an instant or sorcery with one spell ability, cardBuilder.altSacOK).
func altPayLifeExileHand(c *lCost) bool {
	if c.ExileG == nil || !c.ExileHand || c.ExileGN != 1 || c.PayLife != c.ExileGN || c.ExileGBf || c.exileGCard {
		return false
	}
	rest := *c
	rest.ExileG, rest.ExileGN, rest.ExileHand, rest.PayLife = nil, 0, false, 0
	return rest.pureMana()
}
