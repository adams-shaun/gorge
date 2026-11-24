package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"card:Riot", "CardDef.Riot",
		"A printed K:Riot (CR 702.136; kop.KwRiot; gorge cast_etbchoice.go's faceETBChoice): the controller answers an as-enters counter-or-haste election on every face-up battlefield entry (a face-down manifest or cloak poses nothing, CR 708.5): \"Enter with a +1/+1 counter\" puts one +1/+1 counter placed as an entry counter (the would-enter-board settlement, CR 614.12), \"Gain haste\" grants the keyword for as long as the permanent is there (gorge's IntrinsicKeywords append, apply_zone.go). A cast creature's election is asked at the spell's resolution with the spell still on the stack (the same as-enters shape as Unleash's, CR 614.12; dirac kw_riot.go). A face that also prints Unleash refuses (the kernel's pre-entry ask carries one kind), and a GRANTED Riot (a static's AddKeyword$, Rhythm of the Wild) stays refused: gorge's enteringRiot asks for the derived read too, and the kernel reads only the printed field. Ghor-Clan Wrecker, Zhur-Taa Goblin."})
}
