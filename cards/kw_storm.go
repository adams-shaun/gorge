// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwStorm(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Storm",
		"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnCast/Minus1 | MayChooseTarget$ True", has)
}

func init() { registerKeyword(kwStorm, "Storm") }
