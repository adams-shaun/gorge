package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:KwMarkHorsemanship", "ExtKwMark C bit kop.KwMarkHorsemanship",
			"A Pump / PumpAll KW$ Horsemanship (CR 702.31b, until end of turn; Riding Red Hare, Merry-Go-Round's PumpAll): the arena slot bits mechHorsemanship / mechHorsemanshipPost, set, cleared and read as the intimidate mark is (ext:KwMark), as the permanent's CardDef.Evasion EvHorsemanship beside the printed one, both as an attacker's restriction and as a blocker's exception. A Duration$ Permanent grant (Riding the Dilu Horse) is no mark: it is the grant's override record's own Horsemanship (kop.Override.Horsemanship, read by evasionGranted beside the mark, ended with the permanent, lost with the abilities as any layer-6 grant). A Continuous static's AddKeyword$ Horsemanship (Sun Quan, Lord of Wu) is Static.Evasion EvHorsemanship (st:Evasion), read by evasionGranted beside the printed bit."},
	)
}
