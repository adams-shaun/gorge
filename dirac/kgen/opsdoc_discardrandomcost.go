package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"cost:DiscardRandom", "Cost.DiscardRandom (with CostDiscard)",
			"Discard<N/Random> as an activated non-mana ability's or a spell's cost (CR 701.9b, 118.3; Pyromania, Sonic Burst): the CostDiscard part's N cards are picked by the engine's seeded RNG, not chosen. At payment (gorge costObjectAsks, discardAsk's Random arm) the RNG is drawn once per card over the hand in hand order, each pick removed from the list, and nothing is asked; the picks are discarded in pick order. The offer needs a hand of at least N cards besides the spell being cast, so an empty hand withholds the activation. A table byte, not a CostFlags bit (the words are full). A mana ability with the cost, a second discard part, and a filtered one stay refused."})
}
