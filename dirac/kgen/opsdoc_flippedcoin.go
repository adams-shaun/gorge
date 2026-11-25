package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"trig:FlippedCoin", "TrigFlippedCoin (N: TrigNFlipWin / TrigNFlipLose, 0 either)",
			"Whenever a player (Whose: ValidPlayer$, the flipper) wins or loses a coin flip (CR 705.2; gorge trigmatch flippedCoinMatches on effFlipCoin's per-coin result Note). Fired by OpRollDie's coin flip as the coin lands, before its outcome branch; an uncalled NoCall$ flip (RollDieCoinNoCall) fires nothing unless a FlipCoinMod static fixed it (Forge FlipCoinEffect.flipCoin). No slot subscription: the battlefield is scanned (genPrep.anyKwAct, trig_flippedcoin.go). ValidResult$ other than Win/Lose, and a line outside the battlefield, refuse. Chance Encounter, Tavern Scoundrel."},
		opsDoc{"op:BiteExcess", "OpBite A.N BiteExcessCtrl (F condition)",
			"DealDamage ExcessDamage$ TargetedController on a source's-power bite (CR 120.4a, 120.10; gorge effects emitObjectDamageExcess): when the source's power exceeds what is lethal to the bitten creature before the hit (toughness or LethalDamageByPower$ power less marked damage, a planeswalker's loyalty; 1 to a creature from a deathtouch source), the creature is dealt the lethal amount and its controller the rest, in the one damage batch. F is ExcessDamageCondition$, matched against the source (Card.withTrample); a failed match is the plain bite (act_biteexcess.go). Ram Through."})
}
