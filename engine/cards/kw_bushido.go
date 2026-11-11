// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwBushido(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.45a: "Whenever this creature blocks or becomes blocked, it gets
	// +N/+N until end of turn." Forge's CardFactoryUtil builds the same two
	// triggers (Mode$ Blocks | ValidCard$ Card.Self and Mode$ AttackerBlocked |
	// ValidCard$ Card.Self) with one DB$ Pump body; the two dedicated block
	// hooks in engine/rules/trigger_blocks.go queue them, and Pump defaults to
	// until end of turn. N is the printed parameter text verbatim; all 37
	// measured corpus lines are numeric (25x1, 11x2, 1x5) and a bare line
	// expands nothing.
	// param arrives already trimmed (Face.expandKeywords).
	n := param
	if n == "" {
		return
	}
	body := "DB$ Pump | Defined$ Self | NumAtt$ +" + n + " | NumDef$ +" + n
	f.addKeywordTrigger(head, k,
		"Mode$ Blocks | ValidCard$ Card.Self | TriggerDescription$ Bushido", body, has)
	f.addKeywordTrigger(head, k,
		"Mode$ AttackerBlocked | ValidCard$ Card.Self | TriggerDescription$ Bushido", body, has)
}

func init() { registerKeyword(kwBushido, "Bushido") }
