package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"stat:EquipFlash", "StatEquipFlash (Cond2)",
		"The controller may activate Equip abilities (kop.AfEquip) any time they could cast an instant (S:Mode$ CastWithFlash | ValidSA$ Activated.Equip | Caster$ You, CR 702.6; gorge rules equipAtInstantSpeed): canActivate skips the Equip ability's sorcery-speed gate while a battlefield StatEquipFlash static of the activator holds (Cond2: Forge Anew's Condition$ PlayerTurn). Timing only: the cost, targets and once-per-turn gates stand. A ValidCard$ or EffectZone$ rider, another Caster$ and any other condition stay refused; Activated.Loyalty is its own lowering. Leonin Shikari, Forge Anew (which stays refused by its AlternativeCost X blocker). dirac/static_equipflash.go; kgen/static_equipflash.go."})
}
