package kgen

import "testing"

// kvhForms: the value-heads lowerings (MageZero sample
// decks, CARD-CENSUS.md), one carrier per head, checked like paramreadformsForms.
var kvhForms = []formCase{
	{card: "Galvanic Blast", form: "Count$Metalcraft.4.2: three or more artifacts you control (gorge cov3BranchHoldsMetalcraft)", lir: "CountAtLeast({types=0x4 ctrl=1},4,2)"},
	{card: "Anoint with Affliction", form: "TargetedController$Counters.Poison over the one creature target", lir: "PoisonOf(TargetController0)"},
	{card: "Witchstalker Frenzy", form: "PlayerCountPlayers$AttackersDeclared (gorge Engine.AttackersThisTurn)", lir: "amount=AttackersAll"},
	{card: "Lunar Convocation", form: "/Times.X over an SVar: the product (gorge applyCountOpOperand)", lir: "Product(LifeLostThisTurn,LifeGainedThisTurn)"},
	{card: "Refurbished Familiar", form: "/Minus.Remembered$Amount: an inline operand (gorge countOperandValue)", lir: "Sum(OppExtreme,-RememberedCount(ctx))"},
	{card: "Unstoppable Slasher", form: "TriggeredTarget$LifeTotal/HalfUp: (life+1)/2, truncating", lir: "Sum(LifeOf(TriggeredPlayer),1)/2"},
	{card: "Unstoppable Slasher", form: "Card.Self+!HasCounters is PredNoCounters", lir: "Self cmp(17 0 0  )"},
	{card: "Grab the Prize", form: "Discarded$Valid of the spell's one-card discard cost (RefDiscarded); DamageAll ValidPlayers$ Player.Opponent", lir: "1 DamageAll A=RefCount({types=0x0 not=0x1},Discarded)*2 F={players=2}"},
}

func TestValueHeadForms(t *testing.T) { checkForms(t, kvhForms) }

// kfpForms: the filter-props lowerings.
var kfpForms = []formCase{
	{card: "Shoot the Sheriff", form: "Creature.!Outlaw is PredOutlaw negated (gorge outlawMatches)", lir: "cmp(19 0 1  )"},
	{card: "Guardian of the Guildpact", form: "K:Protection:Card.MonoColor is a MonoColor quality (gorge sourceHasQuality)", lir: "protfrom={types=0x0 cmp(20 0 0  )}"},
	{card: "Preacher of the Schism", form: "Attacked$ Player.withMostLife: the opponent's life at least yours, fire-time only", lir: "cond=Value(op1,LifeOf(EachOpp),LifeOf(You))"},
	{card: "Preacher of the Schism", form: "CheckDefinedPlayer$ You.withMostLife: your life at least the opponent's", lir: "cond=Value(op1,LifeOf(You),LifeOf(EachOpp))"},
	{card: "Ruin-Lurker Bat", form: "ValidPlayer$ You.descended: a nontoken permanent card you own entered a graveyard this turn", lir: "ThisTurnEntered({types=0x9f owner=1 NonToken})"},
	{card: "Cut Down", form: "totalPT_LE5 is PredTotalPT (gorge cr-totalpt-filter, 9942071c4)", lir: "cmp(22 2 5  )"},
}

func TestFilterPropForms(t *testing.T) { checkForms(t, kfpForms) }
