package kgen

import (
	"testing"
)

// The cost tokens `costHand` had no arm for, after the parent ticket's
// Draw/DamageYou/Discard arms: an activated `Reveal<1/CARDNAME>` (Karlov's
// Crossbow, the forecast family: the source card is revealed from its
// controller's hand and nothing moves) and an activated `Mill<N>` (Rot Farm
// Skeleton, CR 701.17b: the payer mills N cards from their own library's top).
// The unless-cost window lowers its own Mill and Reveal carriers: Deep Spawn's
// UnlessCost$ Mill<2> (gorge payUnlessCost mills through the shared
// pay.PayMillCost after the pay election, gated pay.MillCostPayable) and
// Priest of the Wakening Sun's UnlessCost$ Reveal<1/Creature.Dinosaur>
// (gorge beginUnlessPayment's "revealcost" pick, settled with a Note and
// nothing moved). The other classes stay refused, named on purpose:
//   - a spell's own reveal cost (Living Destiny, whose body reads the revealed
//     card's mana value);
//   - a mana ability's Mill<1> (Millikin: gorge's manual window DOES pay it,
//     mana_activation.go over pay.ManaCostPayableFull and pay.PayMillCost, so
//     the lowering is its own kernel slice, mana window and gates included);
//   - a trigger's own cost Mill (Ashcoat of the Shadow Swarm: gorge's
//     triggered-cost window keeps every Reveal component and every unlisted
//     component decline-only — cumulative.go triggeredCostComponentsPayable —
//     so the cost is never payable there and the lowering would need a
//     decline-only ask the kernel does not pose yet);
//   - Xyru Specter's UnlessCost$ Reveal<2/Card.Black> and Invasion of the
//     Giants' UnlessCost$ Reveal<1/Giant> lower beside their other blockers
//     (Xyru's GenericChoice without a TempRemember Chooser; Invasion's Effect
//     ForgetOnCast$ beside a static), which keep the cards out on their own.
var revealMillCostForms = []formCase{
	{card: "Karlov's Crossbow", form: "AB$ Effect Cost$ 0 Reveal<1/CARDNAME> (the forecast family)", lir: "RevealSelf"},
	{card: "Rot Farm Skeleton", form: "AB$ ChangeZone Cost$ 2 B G Mill<4>", lir: "Mill4"},
	{card: "Deep Spawn", form: "UnlessCost$ Mill<2> (DB$ Sacrifice | UnlessPayer$ You)", lir: "CS=Mill"},
	{card: "Priest of the Wakening Sun", form: "UnlessCost$ Reveal<1/Creature.Dinosaur> (DB$ GainLife, UnlessSwitched$ True)", lir: "CS=Reveal"},
	{card: "Living Destiny", form: "a spell's own Reveal<1/Creature>, its body reading the revealed card", refused: `reveal cost "Reveal<1/Creature>"`},
	{card: "Millikin", form: "AB$ Mana Cost$ T Mill<1> (a mana ability's mill)", refused: `mill cost "Mill<1>"`},
	{card: "Ashcoat of the Shadow Swarm", form: "a trigger's own Cost$ Mill<4> (AB$ ChangeZone in the body)", refused: "resolution mill cost shape"},
	{card: "Xyru Specter", form: "UnlessCost$ Reveal<2/Card.Black> beside DB$ GenericChoice (measured)", refused: "GenericChoice"},
	{card: "Invasion of the Giants", form: "UnlessCost$ Reveal<1/Giant> beside DB$ Effect ForgetOnCast$ (measured)", refused: "ForgetOnCast"},
	{card: "Tresserhorn's Lord, Returned", form: "UnlessCost$ Sac<3/Creature> PayLife<3> Draw<3/Player.targetedBy> (measured)", refused: "resolution cost shape"},
}

func TestRevealMillCostForms(t *testing.T) {
	checkForms(t, revealMillCostForms)
}
