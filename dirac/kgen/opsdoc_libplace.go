package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:LibPlace", "OpExt ExtLibPlace (C kop.PlaceArmTop / PlaceArmBottom / PlaceAsk)",
			"The owner's CR 401.4 arrangement of cards one effect puts at a library position together (gorge effects.libraryPlacementOrder: a KArrange of Kind \"library_placement\", Min == Max == the cards offered, the first pick nearest the top, asked of the cards' OWNER, not the controller). It brackets the move: the arm (before) counts, per owner in first-placed order, the cards the next moves put into libraries; the ask (after) poses one ChArrange per owner with two or more of them, over the cards at the top (arm top) or the bottom (arm bottom) of that library, picks until all are ordered, and reorders the group in place. Nothing is asked when no owner received two cards. The tally lives in the resolution's scratch arm, no State byte. Emitted around a ChangeZoneAll sweep into a library (no RandomOrder$, no Shuffle$ True) and a multi-target ChangeZone to the top or bottom (no RandomOrder$, no Shuffle$ True without NoShuffle$). Elixir of Immortality's graveyard sweep, Bow of Nylea's \"up to four target cards on the bottom in any order\"."})
}
