package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"stat:IgnoreLandWalk", "StatIgnoreLandWalk (Static.Filter, Static.Landwalk)",
			"S:Mode$ IgnoreLandWalk (CR 702.14: \"creatures with swampwalk can be blocked as though they didn't have swampwalk\"): a creature can block an attacker as though the landwalk line the static names were absent. Static.Landwalk is the land filter of the line it names (ValidKeyword$ Landwalk:<spec>, one land clause; 0 = every landwalk line), Static.Filter the would-be blocker (ValidBlocker$, read against the static's controller and source; 0 = any blocker), Cond2 the static's gate. The kernel asks per link of the attacker's landwalk filter (each link is one landwalk line) whether a static ignores it for the blocker before it reads the defending player's lands, over the printed landwalk, a static's grant and a Pump's record alike (gorge combat.landwalkIgnored). Crevasse, Deadfall, Great Wall, Lord Magnus, Quagmire, Staff of the Ages, Street Savvy, Undertow, Ur-Drago, Gosta Dirk."},
		opsDoc{"ext:KwMarkSkulk", "ExtKwMark C bit kop.KwMarkSkulk",
			"A Pump / PumpAll KW$ Skulk (CR 702.118a, until end of turn; MacCready, Lamplight Mayor, The Master, Mesmerist): the arena slot bits mechSkulk / mechSkulkPost, set, cleared and read as the intimidate mark is (ext:KwMark), as the permanent's CardDef.Evasion EvSkulk beside the printed one. A Continuous static's AddKeyword$ Skulk (Behind the Scenes, Skeleton Key) is Static.Evasion EvSkulk (st:Evasion); the withSkulk filter word reads either (pred:Word7 sub-kind 40, kop.W7WithSkulk)."},
		opsDoc{"stat:LandwalkGrantForms", "Static.Landwalk / Override.Landwalk",
			"A granted landwalk (CR 702.14) in every spelling kgen lowers: a Continuous static's AddKeyword$ Landwalk:<spec> over an Aura's enchanted creature (Burrowing, Fishliver Oil), an Equipment's equipped creature or a global filter is Static.Landwalk beside the static's filter; a landwalk beside a set-P/T part of an override static (Halimar Wavewatch's level band) rides a companion Continuous static over the same filter and condition; a PumpAll KW$ Landwalk:<spec> (Nature's Cloak) is an end-of-turn OverrideAll record carrying Override.Landwalk over the sweep. YourBasic, ChosenType and the bare shared Landwalk copy (SharedKeywordsZone$) stay refused."},
	)
}
