package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"card:Phasing", "CardDef.Phasing",
		"A printed bare K:Phasing (CR 702.26a; gorge rules/turn.go finishUntapStep): a phased-in permanent of it phases out as its controller's untap step begins, before the untap action (its summoning sickness already ended with the turn change), and a phased-out one phases in at the next untap step, each permanent read once in entry order so an Aura that went out with its host (CR 702.26g) is not toggled by the scan; a face-down permanent or one that lost all abilities has no keyword. The kernel's untap-step phasing action reads it beside the phase-in scan (dirac/kw_phasing.go phaseUntap, behind prep.phasing, which a printed Phasing switches on). A granted Phasing, and a card with a phase-in or phase-out trigger, stay refused. Breezekeeper, Teferi's Drake, Tolarian Drake."})
}
