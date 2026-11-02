// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwRenown(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.112a: "if this creature isn't renowned" is an intervening if (CR 603.4).
	f.addKeywordTrigger(head, k,
		"Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | IsPresent$ Card.Self+!IsRenowned | PresentDefined$ Self | TriggerDescription$ Renown",
		"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | Renown$ "+param, has)
}

func init() { registerKeyword(kwRenown, "Renown") }
