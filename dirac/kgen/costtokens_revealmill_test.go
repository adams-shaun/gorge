package kgen

import (
	"testing"
)

// The cost tokens `costHand` had no arm for, after the parent ticket's
// Draw/DamageYou/Discard arms: an activated `Reveal<1/CARDNAME>` (Karlov's
// Crossbow, the forecast family: the source card is revealed from its
// controller's hand and nothing moves) and an activated `Mill<N>` (Rot Farm
// Skeleton, CR 701.17b: the payer mills N cards from their own library's top).
// The other Reveal<...> and Mill<...> classes stay refused, named on purpose:
// a spell's own reveal cost (Living Destiny, whose body reads the revealed
// card's mana value) and a mana ability's Mill<1> (Millikin, whose cost shape
// kop.ManaAbilityHonoured does not list).
var revealMillCostForms = []formCase{
	{card: "Karlov's Crossbow", form: "AB$ Effect Cost$ 0 Reveal<1/CARDNAME> (the forecast family)", lir: "RevealSelf"},
	{card: "Rot Farm Skeleton", form: "AB$ ChangeZone Cost$ 2 B G Mill<4>", lir: "Mill4"},
	{card: "Living Destiny", form: "a spell's own Reveal<1/Creature>, its body reading the revealed card", refused: `reveal cost "Reveal<1/Creature>"`},
	{card: "Millikin", form: "AB$ Mana Cost$ T Mill<1> (a mana ability's mill)", refused: `mill cost "Mill<1>"`},
}

func TestRevealMillCostForms(t *testing.T) {
	checkForms(t, revealMillCostForms)
}
