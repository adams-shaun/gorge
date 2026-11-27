package kgen

// playThisTurnCastHand reports Twinning Glass's population: a nonland card in
// the controller's hand sharing a name with a spell cast this turn (Valid$
// Card.nonLand+sharesNameWith ThisTurnCast, ValidZone$ Hand). gorge's effPlay
// walks only the resolving controller's hand for it (playValidReadsOtherHand
// names no player referent), so the YouOwn/YouCtrl the hand arm otherwise
// asks for would filter nothing; the referent is the kernel's
// FwSharesNameThisTurnCast.
func playThisTurnCastHand(valid, vzone string) bool {
	return vzone == "Hand" && valid == "Card.nonLand+sharesNameWith ThisTurnCast"
}
