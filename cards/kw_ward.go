// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwWard(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// Ward is a becomes-target trigger. Ward$ lets the matcher exclude
	// the permanent's controller; the effect counters the targeting
	// spell or ability unless that player pays the printed cost.
	f.addKeywordTrigger(head, k, "Mode$ BecomesTarget | ValidTarget$ Card.Self | Ward$ True | TriggerDescription$ Ward",
		"DB$ Ward | UnlessCost$ "+param, has)
}

func init() { registerKeyword(kwWard, "Ward") }
