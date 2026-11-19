package kgen

func init() {
	opsEntries = append(opsEntries, opsDoc{"ext:Clash", "OpExt ExtClash ()",
		"Clash with an opponent (CR 701.31; gorge effClash): with two or more living opponents the controller first chooses which (raPodClashOpp, ChGenPlayer, mode = the seat, AliveFrom(0) order; CR 701.31a), held in the resolution's chosen-player byte until the placements are answered; then both players reveal their top card, then the resolving controller and after them the opponent (APNAP order, CR 101.4) each put their own revealed card on the top or the bottom of their library (one ChGenClash ask each, mode 1 top / 0 bottom, over the revealed card; asked only of a player whose library had a card). The player whose card has the strictly higher mana value wins; a tie, or two empty libraries, has no winner. The verdict is left in the resolution's result slot (ExRollResult): 1 the controller won, 2 otherwise; WinSubAbility$ / OtherwiseSubAbility$ lower as blocks guarded by it. Adder-Staff Boggart, Bog Hoodlums, Broken Ambitions, Fire Juggler. dirac/act_clash.go."})
}
