package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:RingTemptsYou", "OpExt ExtRingTemptsYou (Obj the tempted player)",
			"The Ring tempts the player Obj (the resolving controller; CR 701.54; gorge effRingTemptsYou): their tempted count rises by one and a creature they control becomes (or stays) their Ring-bearer. The creatures they control are taken in battlefield order: none, and the count still rises with no bearer (CR 701.54d); one, designated unasked; two or more, the player picks (ChGenRingBearer, gorge's `ring_bearer` KChoose, the current bearer first when it is still theirs). TrigRingTemptsYou fires with the bearer as its object. A table with the op also keeps the Ring emblem: the bearer is legendary and can't be blocked by a creature of greater power, and while the count is 2 / 3 / 4 or more a source-less trigger the player controls (the monarch's shape) loots when the bearer attacks, makes each blocker's controller sacrifice it at end of combat (one trigger per blocker), and drains each opponent for 3 when the bearer deals combat damage to a player. The designation ends when the bearer leaves the battlefield or another player gains control of it. State is one cold region (genPrep.anyRing). dirac/act_ring.go. Call of the Ring, Boromir, Warden of the Tower, Claim the Precious, Samwise the Stouthearted."},
		opsDoc{"trig:RingTemptsYou", "TrigRingTemptsYou (Whose, Filter)",
			"Whenever the Ring tempts a player (Whose: the controller, an opponent or anyone; gorge Mode$ RingTemptsYou, trigmatch ringTemptsMatches on events.RingTemptsYou): fired by ExtRingTemptsYou. Filter is ValidCard$, read on the creature chosen as the Ring-bearer with the listener as \"you\" and the source (Creature.YouCtrl+Other: a creature other than this one); a temptation that chose no creature never fires a ValidCard$ trigger. The bearer is the trigger's object. No slot subscription: the battlefield is scanned in listener order. dirac/act_ring.go. Call of the Ring, Sauron, the Dark Lord, Sméagol, Helpful Guide."},
		opsDoc{"ex:RingTempted", "ExRingTempted",
			"How many times the Ring has tempted the reading controller (gorge PlayerCountPropertyYou$RingTemptedYou, the uncapped Player.RingTempted; saturating at 255). dirac/act_ring.go. Frodo, Adventurous Hobbit."},
		opsDoc{"pred:Ringbearer", "PredRingbearer",
			"The filter property IsRingbearer (CR 701.54e; gorge wordRingBearer): a battlefield permanent that is its controller's Ring-bearer. Off the battlefield nothing is. dirac/act_ring.go. One Ring to Rule Them All, Frodo, Adventurous Hobbit."},
	)
}
