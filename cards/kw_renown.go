// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwRenown(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k,
		"Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | TriggerDescription$ Renown",
		"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | Renown$ "+param, has)
}

func init() { registerKeyword(kwRenown, "Renown") }
