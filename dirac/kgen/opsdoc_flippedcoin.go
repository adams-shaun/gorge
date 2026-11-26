package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"trig:FlippedCoin", "TrigFlippedCoin (N: TrigNFlipWin / TrigNFlipLose, 0 either)",
			"Whenever a player (Whose: ValidPlayer$, the flipper) wins or loses a coin flip (CR 705.2; gorge trigmatch flippedCoinMatches on effFlipCoin's per-coin result Note). Fired by OpRollDie's coin flip as the coin lands, before its outcome branch; an uncalled NoCall$ flip (RollDieCoinNoCall) fires nothing unless a FlipCoinMod static fixed it (Forge FlipCoinEffect.flipCoin). No slot subscription: the battlefield is scanned (genPrep.anyKwAct, trig_flippedcoin.go). ValidResult$ other than Win/Lose, and a line outside the battlefield, refuse. Chance Encounter, Tavern Scoundrel."})
}
