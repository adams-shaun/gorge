package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:ChoosePlayerRes", "OpExt ExtChoosePlayerRes (C bit 0 the Player.!IsRemembered pool)",
			"A player chosen mid-resolution (gorge effChoosePlayer with Defined$ You, Choices$ Player or Player.!IsRemembered and RememberChosen$ True; Gluntch, the Bestower): the resolving controller chooses one living player, one ChGenPlayer option per player in AliveFrom order (mode = the seat), at any seat count. The answer becomes the resolution's chosen player (RefChosenPlayerRes) and joins its remembered players; with C bit 0 the pool leaves out the players already remembered. A pool of one is taken unasked (the oracle answers a single option in place), an empty pool leaves no chosen player. Both live in resCtx.cpl, so the Cleanup ClearRemembered$ that ends the chain also forgets the players. dirac/act_chooseplayerres.go."},
		opsDoc{"ref:ChosenPlayerRes", "RefChosenPlayerRes",
			"A player ref: the player this resolution's last ExtChoosePlayerRes chose (gorge Defined$ ChosenPlayer over Ctx.Chosen). None before the first choice or after an empty pool. A Draw, a CreateToken owner, a PutCounter pick's chooser and a PutCounter's Placer$ (OpPutCounter W: the counters' adder, read by the counter replacements and the added-this-turn ledger) read it. A Choices$ Creature.ControlledBy ChosenPlayer beside Chooser$ ChosenPlayer lowers to the chooser's own side (YouCtrl)."})
}
