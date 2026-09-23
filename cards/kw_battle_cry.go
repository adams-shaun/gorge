// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwBattleCry(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.33: when this creature attacks, each other attacking creature
	// gets +1/+0 until end of turn. PumpAll snapshots the attacking set as
	// this trigger resolves; StrictlyOther is relative to the trigger source.
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Battle cry",
		"DB$ PumpAll | ValidCards$ Creature.attacking+StrictlyOther | NumAtt$ +1 | NumDef$ 0", has)
}

func init() { registerKeyword(kwBattleCry, "Battle cry") }
