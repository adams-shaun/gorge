package oracle

import "github.com/adams-shaun/gorge/dirac"

// The Count$ heads kgen lowers whose amount needs no stack entry: a count of
// the objects a filter matches on the board or in a zone, the life totals, the
// turn and hand reads. Count$DamageOppsTakenThisTurn is left out: its lowering
// turns on a state bank the game's table lacks (NewProbeSet refuses it).
// twoSeatHeads are the heads kgen lowers over "you and the opponent" (a sum of
// the two hand sizes), which a pod's other seats make a different number.
var twoSeatHeads = map[string]bool{"Count$ValidHand Card": true}

func init() {
	for _, h := range []string{
		"Count$Valid Creature.YouCtrl", "Count$Valid Creature.OppCtrl", "Count$Valid Creature",
		"Count$Valid Land.YouCtrl", "Count$Valid Land.OppCtrl", "Count$Valid Permanent.YouCtrl",
		"Count$Valid Card.tapped+YouCtrl", "Count$Valid Creature.attacking",
		"Count$Valid Creature.token+YouCtrl", "Count$Valid Creature.powerGE3+YouCtrl",
		"Count$ValidHand Card.YouOwn", "Count$ValidHand Card.OppOwn",
		"Count$ValidGraveyard Card.YouOwn", "Count$ValidGraveyard Creature.YouOwn", "Count$ValidGraveyard Card.OppOwn",
		"Count$ValidExile Card.YouOwn",
		"Count$YourLifeTotal", "Count$CardsInYourHand", "Count$YourStartingLife", "Count$ValidHand Card", "Count$ValidHand Land.YouOwn",
		"Count$ValidLibrary Card.YouOwn", "Count$YourExtraTurns", "Count$TopOfLibraryCMC",
		"Count$YourCountersEnergy", "Count$YourCountersExperience", "Count$CardPower", "Count$CardToughness",
		"PlayerCountOpponents$Amount", "PlayerCountOpponents$HighestLifeTotal", "PlayerCountOpponents$LowestLifeTotal",
		"PlayerCountOpponents$HighestCardsInHand", "PlayerCountOpponents$LowestCardsInHand", "PlayerCountOpponents$HighestCardsInGraveyard",
		"Count$LifeYouGainedThisTurn", "Count$LifeYouLostThisTurn", "Count$LifeOppsLostThisTurn",
		"Count$YouDescendedThisTurn",
		"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature", "Count$Valid Creature.YouCtrl/Plus.1", "Count$Valid Creature.YouCtrl/Twice",
	} {
		RegisterCountAtom(CountAtom{Name: "count:" + h, GorgeHead: h, TwoSeat: twoSeatHeads[h]})
	}
	// each player's turn count is kept only for a table that reads it
	RegisterCountAtom(CountAtom{Name: "count:Count$YourTurns", GorgeHead: "Count$YourTurns", Needs: dirac.BankYourTurns})
}
