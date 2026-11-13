package kgen

import "testing"

// blockTaxForms: S:Mode$ CantBlockUnless lowers to kop.StatBlockTax when its
// Cost$ is generic mana (a literal, an inline Count$ or an SVar of the face;
// lower_blocktax.go), and every other shape gorge's blockPairCharge prices
// stays refused by name with its reason.
var blockTaxForms = []formCase{
	{card: "Archangel of Tithes", form: "IsPresent$ Card.Self+attacking | ValidCard$ Creature | Cost$ 1"},
	{card: "Qal Sisma Behemoth", form: "ValidCard$ Card.Self | Cost$ 2, no Attacker$", lir: "static CantBlockUnless F={types=0x0 Self} F2=nil"},
	{card: "Qal Sisma Behemoth", form: "the literal charge", lir: "amount=2"},
	{card: "Myr Prototype", form: "Cost$ Y, Y = its +1/+1 counters", lir: "amount=CountersOn(Self,P1P1)"},
	{card: "Hipparion", form: "ValidCard$ Card.Self | Attacker$ Creature.powerGE3 | Cost$ 1"},
	{card: "Awesome Presence", form: "no ValidCard$ (every creature) | Attacker$ Creature.AttachedBy | Cost$ 3"},
	{card: "Cowed by Wisdom", form: "ValidCard$ Creature.AttachedBy | Cost$ Y, Y = Count$ValidHand Card.YouOwn"},
	{card: "Oppressive Rays", form: "ValidCard$ Creature.AttachedBy | Cost$ 3, beside a RaiseCost Type$ Ability",
		refused: `RaiseCost Type "Ability"`, lir: "static CantBlockUnless F={types=0x2 EquippedBy}"},
	{card: "Heat Wave", form: "Cost$ PayLife<1>", refused: `CantBlockUnless Cost$ "PayLife<1>"`},
	{card: "Hollow Warrior", form: "Cost$ tapXType<1/Creature.!blocking>", refused: `CantBlockUnless Cost$ "tapXType<1/Creature.!blocking>"`},
}

// TestBlockTaxForms pins the lowered and refused CantBlockUnless shapes.
func TestBlockTaxForms(t *testing.T) { checkForms(t, blockTaxForms) }
