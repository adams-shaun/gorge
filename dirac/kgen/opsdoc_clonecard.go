package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ex:CardBasePower", "ExCardBasePower",
			"The evaluating source's base power: the layer-7b value before counters and pumps (CR 613.4), a copy's copied power and a set-power effect among it (gorge Count$CardBasePower, h.Chars(Source).BasePower); a source off the battlefield reads its printed power. dirac/value_basepower.go. Curie, Emergent Intelligence."},
		opsDoc{"ext:CloneCopyOffBattlefield", "ExtCloneCopy B.N kop.CloneTmplCard / kop.CloneTmplSac (with CloneGrant for AddTriggers$)",
			"DB$ Clone whose template is a card, not a permanent, copied as its printed card (CR 400.7; gorge copies the source object's printed face): CloneTmplCard (TgtZone$ Graveyard, an until-end-of-turn copy; Shifting Woodland) the card target slot B.N>>CloneTmplShift&3 names, CloneTmplSac (Defined$ Exiled; Curie, Emergent Intelligence) the one card the activation's own Exile<1/Spec> cost exiled, read from the entry's sacrifice snapshot (snapSacPick), the cost a nontoken artifact or creature. The template has no copy exceptions of its own to inherit. A permanent copy records no source object on its marker (re-bases read the locked card). CloneGrant beside AddTriggers$: the override record after C grants the lowered latent trigger of this card (a copy of a copy has it twice, CR 707.9b). AddTypes$ rides the exceptions' override (Saheeli, Sublime Artificer). dirac/copy_clonecard.go."},
	)
}
