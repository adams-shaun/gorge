package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"pred:MonarchCtrl", "PredMonarchCtrl (PredN 0 / 1)",
			"PredN 0: a battlefield permanent its controller, the monarch, controls (gorge `ControlledBy Player.isMonarch`: with no monarch the referent is unbound and nothing matches); PredN 1: a permanent whose controller is not the monarch (gorge `ControlledBy EnchantedController.!isMonarch`, lowered only beside EnchantedBy, where the enchanted controller is the candidate's own; with no monarch it matches). Never matches off the battlefield (kgen refuses it there). Reads genState.mon (dirac/filter_monarch.go). Azure Fleet Admiral's CantBlockBy ValidBlocker$; Fall from Favor's Untap replacement."},
		opsDoc{"ex:MonarchAny", "ExMonarch N 2 (kop.MonarchAny)",
			"Sub[0] if some player still in the game is the monarch, else Sub[1] (gorge `PlayerCountPlayers$HasPropertyisMonarch`, the 0-or-1 count of living players holding the designation). Archivist of Gondor's and Crown of Gondor's \"if there is no monarch\" CheckSVar$. Also the Phase trigger Whose kop.WhoseMonarch (ValidPlayer$ Player.isMonarch: the active player is the monarch; dirac/filter_monarch.go monarchTurnIs), Archivist of Gondor's \"the monarch's end step\"."})
}
