package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ex:StaticExileKw", "ExStaticExileKw (F, N)",
			"Exiled cards matching F, every player's exile (F.Owner narrows), that print the keyword N names: N below 32 is a KwMask bit index, N 32 and up the protection colour (N-32, kop.ExileKwProtN order) in the card's printed ProtColors. The printed card is read (CR 400.7), never a layer result. It is the per-keyword gate of a `SharedKeywordsZone$ Exile` static (Rayami, First of the Fallen), compared at least 1 in the static's Cond. Gorge statSharedKeywords."})
}
