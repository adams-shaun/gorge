package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"card:Flanking", "CardDef.Flanking",
		"A printed bare K:Flanking (CR 702.25a): gorge's kwFlanking expansion, one AttackerBlockedByCreature trigger (ValidCard$ Card.Self, ValidBlocker$ Creature.withoutFlanking; its Pump -1/-1 body names the blocker, TriggeredBlockerLKICopy), is lowered as an ordinary become-blocked trigger, and CardDef.Flanking records the printed keyword for the withoutFlanking filter word (pred:Word7 sub-kind 46, kop.W7WithoutFlanking, dirac/filter_flanking.go): a battlefield blocker has flanking when its card prints it, unless it is face down or lost all abilities; off the battlefield the printed card counts. A granted flanking, the withFlanking word and a second K:Flanking line stay refused (gorge's trigger_blocks.go derived-instance count has no kernel read). Benalish Cavalry, Zhalfirin Knight."})
}
