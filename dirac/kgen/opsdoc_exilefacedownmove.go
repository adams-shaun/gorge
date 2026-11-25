package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"zone:ExileFaceDown", "OpExt ExtExileFaceDown",
		"A ChangeZone or ChangeZoneAll into exile with ExileFaceDown$ True, no WithMayLook$, Foretold$ or DefinedExiler$, beside RememberChanged$ True and first in its program: right after the move each remembered card in exile is marked exiled face down by the resolving source, nobody the looker (gorge applyMoveFaceDown's blind exiled_with_face_down marker; the Dig face-down mark, dirac/zone_exilefacedown.go). The same blind mark covers a Dig of the controller's own library (Defined$ You, no Effect after it; lower_blinddig.go ownBlindDig). An unremembered face-down move refuses. Induced Amnesia, Triumph of Saint Katherine, Bomat Courier, Connecting the Dots."})
}
