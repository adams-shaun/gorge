// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwProwess(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ You | TriggerDescription$ Prowess",
		"DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +1", has)
}

func init() { registerKeyword(kwProwess, "Prowess") }
