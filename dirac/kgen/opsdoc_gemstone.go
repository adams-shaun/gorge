package kgen

func init() {
	opsCardFields = append(opsCardFields, opsDoc{"card:LeylineGem", "CardDef.LeylineGem", "The Leyline ask's Gemstone shape (CR 103.6a; gorge rules/opening_hand.go): a MayEffectFromOpeningHand gated !PlayFirst whose action moves the card from hand to battlefield, puts LeylineCtrN counters of LeylineCtr on it and then exiles a card from that hand (Mandatory$, ChangeNum$ 1). The starting player is not asked; a yes is followed by the ChGenOpeningExile pick over the player's hand in hand order (gorge's opening_exile), before the next seat's ask. Kernel turn_gemstone.go. Gemstone Caverns."})
}
