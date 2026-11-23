package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ex:TotalCommanderCastFromCommandZone", "ExTotalCommanderCastFromCommandZone",
			"The reading controller's casts of its own commanders from the command zone this game, the sum over its commander slots (a partner pair together), saturating at 255 as the CR 903.8 tax counter does; 0 for a seat with no commander or outside a Commander game (gorge Count$TotalCommanderCastFromCommandZone, Host.CommanderCastsFromCommandZone: the same criteria as the commander tax). dirac/value_cmdcast.go. Captain Vargus Wrath, Commander's Insignia, Echo Storm, Empyrial Storm, Font of Magic, Fury Storm, Hatut Zeraze Strike Force, Jirina Kudro, Jyoti Moag Ancient, The Swarmlord, Vanguard of the Restless."})
}
