package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:Radiation", "OpExt ExtRadiation (Obj player(s), B amount)",
		"Each player Obj names gets B rad counters (gorge effRadiation: a PlayerCounterChange \"RAD\" through the AddCounter replacements; kept per seat in a cold array, genPrep.anyRad). At the beginning of the precombat main phase of a player with a rad counter, after the step's printed triggers and the Sagas' lore counters, a source-less trigger that player controls is queued (CR 728.1; gorge turn.go RadiationDrain). It resolves as gorge effRadiationDrain: the player mills as many cards as they have rad counters, one at a time from the top, and for each nonland card milled loses 1 life and removes a rad counter. Proliferate offers a player holding rad counters. Feral Ghoul, Nuclear Fallout, Contaminated Drink. dirac/act_radiation.go."})
}
