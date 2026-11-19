package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:Encore", "OpExt ExtEncore (Obj the copied card, B.N CopyFlags, C copy exceptions)",
			"AB$ Encore (CR 702.141a; gorge effEncore): for each opponent, from the resolving controller in turn order (AliveFrom), one token copy of Obj, read as OpCopyAsTokenEx reads Obj, B.N and C (the haste is its PumpKeywords$ EOT grant, the end-step sacrifice its CpSacAtEndDly rider, every copy after the first joining the first one's trigger: gorge registers one group). Each copy attacks its opponent this turn if able: the arena bits mechAttacksIfAble and mechAttacksPlayer and the seat in mechEncSeat0/1 (gorge EncoreAttackDefender), so the attackers decision offers it that player alone while that player is in the game and the pair can be declared (CR 508.1d). The CreateToken replacements are read from the permanents present as the walk begins; a copy that parks an ask, or a CreateToken rewriter, stalls. Impulsive Pilferer, Kinsbaile Courier, Angel of Indemnity. dirac/kw_encore.go."})
}
