package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"st:FunctionsInGraveyard", "StFunctionsInGraveyard on StatContinuous",
			"A Continuous keyword or landwalk grant that works while its source card is in its owner's graveyard and not on the battlefield (EffectZone$ Graveyard, CR 113.6 and 604.2; the Mirage Incarnations: Anger, Wonder, Filth, Valor). The card is no battlefield static (it never joins statM); the kernel yields the static from each seat's graveyard (eachGyStatic) with the owner as its controller and the layer stamp its card carried off the battlefield (grantTs; static_gystamp.go), under Cond2 (the IsPresent$ gate). Only a lone plain keyword grant is lowered; a face beside another static, or any other grant, stays refused."},
	)
}
