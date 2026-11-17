// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwChampion expands Champion an [object] (CR 702.72a): "When this permanent
// enters, sacrifice it unless you exile another [object] you control" and
// "When this permanent leaves the battlefield, return the exiled card to the
// battlefield under its owner's control."
//
// One ETB trigger (the Banisher Priest shape: ChangesZone to the battlefield,
// ValidCard$ Card.Self). Its effect is the linked exile-until-leaves
// ChangeZone (Hidden$ True, you choose, Duration$ UntilHostLeavesPlay, which
// also supplies the leaves-the-battlefield return), remembering what moved;
// its sub-ability sacrifices the host when nothing was remembered, then
// clears the memory. The choice is not Mandatory$, so declining to exile
// sacrifices the champion.
//
// The param is one type word ("Goblin", "Creature") or a comma list of them
// ("Goblin,Shaman", "Treefolk,Warrior": measured over every corpus line), and
// each word becomes its own Other+YouCtrl alternative.
func kwChampion(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if param == "" || has("T", k) {
		return
	}
	// Each comma-separated word becomes one alternative (no strings.* here:
	// the engine/cards strings ratchet is full).
	types, word := "", ""
	flush := func() {
		if word == "" {
			return
		}
		if types != "" {
			types += ","
		}
		types += word + ".Other+YouCtrl"
		word = ""
	}
	for j := 0; j < len(param); j++ {
		switch c := param[j]; c {
		case ',', ' ':
			flush()
		default:
			word += string(c)
		}
	}
	flush()
	if types == "" {
		return
	}
	sac := "__kwChampionSac"
	cleanup := "__kwChampionCleanup"
	f.setSVar(cleanup, "DB$ Cleanup | ClearRemembered$ True")
	f.setSVar(sac, "DB$ Sacrifice | Defined$ Self | SacValid$ Self | ConditionDefined$ Remembered | ConditionPresent$ Card | ConditionCompare$ EQ0 | SubAbility$ "+cleanup)
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Champion "+param,
		"DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ChangeType$ "+types+
			" | ChangeNum$ 1 | Hidden$ True | Chooser$ You | RememberChanged$ True | Duration$ UntilHostLeavesPlay | SubAbility$ "+sac, has)
}

func init() { registerKeyword(kwChampion, "Champion") }
